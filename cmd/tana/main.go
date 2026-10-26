package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ivanpasq/tana/internal/repl"
	"github.com/ivanpasq/tana/internal/store"
)

const defaultDataFile = "tana.db"

func main() {
	dataPtr := flag.String("data", defaultDataFile, "the db log location file")
	flag.Parse()

	s, err := store.Open(*dataPtr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "there was an error starting tana: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Starting main repl...")
	err = repl.Run(s, os.Stdin, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "there was an error: %v\n", err)
		os.Exit(1)
	}

	if err := s.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "error while closing tana: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Exiting.")
}
