package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/ivanpasq/tana/internal/repl"
	"github.com/ivanpasq/tana/internal/store"
)

const (
	defaultDataFile = "tana.db"
	defaultAddr     = "localhost:6380"
)

func main() {
	dataPtr := flag.String("data", defaultDataFile, "the db log location file")
	addrPtr := flag.String("addr", defaultAddr, "address to expose tana")
	flag.Parse()

	s, err := store.Open(*dataPtr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "there was an error starting tana: %v", err)
		os.Exit(1)
	}

	l, err := net.Listen("tcp", *addrPtr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error while starting tana listener: %v\n", err)
		os.Exit(1)
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-ch
		fmt.Fprintf(os.Stderr, "shutting down...\n")
		if err := l.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error closing listener: %v\n", err)
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

		go startRepl(s, conn)
	}

	if err := s.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "error while closing tana: %v\n", err)
	}
}

func startRepl(s *store.Store, conn net.Conn) {
	defer func() { _ = conn.Close() }()

	err := repl.Run(s, conn, conn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error while running tana: %v\n", err)
	}
}
