package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/ivanpasq/tana/internal/repl"
	"github.com/ivanpasq/tana/internal/store"
)

const (
	defaultDataFile = "tana.db"
	defaultAddr     = "localhost:6380"
)

type connTracker struct {
	closing bool
	clients map[net.Conn]struct{}
	mu      sync.Mutex
}

func newConnTracker() *connTracker {
	return &connTracker{
		clients: make(map[net.Conn]struct{}),
	}
}

func (ct *connTracker) Set(k net.Conn) bool {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	if ct.closing {
		return false
	}

	ct.clients[k] = struct{}{}
	return true
}

func (ct *connTracker) Del(k net.Conn) bool {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	if _, ok := ct.clients[k]; ok {
		delete(ct.clients, k)
		return true
	}

	return false
}

func (ct *connTracker) ClosingConnection() {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	ct.closing = true
}

func main() {
	dataPtr := flag.String("data", defaultDataFile, "the db log location file")
	addrPtr := flag.String("addr", defaultAddr, "address to expose tana")
	flag.Parse()

	s, err := store.Open(*dataPtr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "there was an error starting tana: %v\n", err)
		os.Exit(1)
	}

	connClients := newConnTracker()

	l, err := net.Listen("tcp", *addrPtr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error while starting tana listener: %v\n", err)
		os.Exit(1)
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	var wg sync.WaitGroup

	go func() {
		<-ch

		connClients.ClosingConnection()
		fmt.Fprintf(os.Stderr, "shutting down...\n")

		if err := l.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error closing listener: %v\n", err)
		}

		connClients.mu.Lock()
		defer connClients.mu.Unlock()

		fmt.Fprintf(os.Stderr, "closing %d open connections...\n", len(connClients.clients))

		for conn := range connClients.clients {
			if _, err := conn.Write([]byte("remote host is closing the connection...\n")); err != nil {
				fmt.Fprintf(os.Stderr, "close connection notification error: %v\n", err)
			}

			if err := conn.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "error closing connection %v: %v\n", conn.RemoteAddr(), err)
			} else {
				fmt.Fprintf(os.Stderr, "disconnected client %v\n", conn.RemoteAddr())
			}
		}
	}()

	fmt.Fprintf(os.Stderr, "listening on %s\n", *addrPtr)

	for {
		conn, err := l.Accept()
		if errors.Is(err, net.ErrClosed) {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "error accepting new connection: %v\n", err)
			continue
		}

		if !connClients.Set(conn) {
			if _, err := conn.Write([]byte("remote host is closing connection...\n")); err != nil {
				fmt.Fprintf(os.Stderr, "error while notifying conn %v about shutdown: %v\n", conn.RemoteAddr(), err)
			}
			if err := conn.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "error closing connection during shutdown: %v\n", err)
			}
			break
		}
		wg.Add(1)
		fmt.Fprintf(os.Stderr, "client connected %v\n", conn.RemoteAddr())

		go startRepl(s, conn, connClients, &wg)
	}

	wg.Wait()

	if err := s.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "error while closing tana: %v\n", err)
	}
}

func startRepl(s *store.Store, conn net.Conn, ct *connTracker, wg *sync.WaitGroup) {
	defer func() {
		ra := conn.RemoteAddr()
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			fmt.Fprintf(os.Stderr, "error closing connection for %v: %v\n", ra, err)
		}
		ct.Del(conn)
		fmt.Fprintf(os.Stderr, "connection %v closed\n", ra)
		wg.Done()
	}()

	err := repl.Run(s, conn, conn)
	if errors.Is(err, net.ErrClosed) {
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error while running tana: %v\n", err)
	}
}
