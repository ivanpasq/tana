package main

import (
	"fmt"
	"os"

	"github.com/ivanpasq/tana/internal/repl"
	"github.com/ivanpasq/tana/internal/store"
)

func main() {
	tana := store.New()

	fmt.Println("Starting main repl...")
	err := repl.Run(tana, os.Stdin, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "there was an error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Exiting.")
}
