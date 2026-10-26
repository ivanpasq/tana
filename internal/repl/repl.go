package repl

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ivanpasq/tana/internal/store"
)

func Run(s *store.Store, r io.Reader, w io.Writer) error {
	sc := bufio.NewScanner(r)

	for sc.Scan() {
		input := sc.Text()

		cmd, args, err := parseCmd(input)
		if err != nil {
			fmt.Fprintln(w, "ERR", err)
			continue
		}
		if cmd == "" {
			continue
		}

		switch strings.ToLower(cmd) {
		case "get":
			if len(args) != 1 {
				fmt.Fprintln(w, "ERR GET requires one argument")
				continue
			}

			key := args[0]
			val, ok := s.Get(key)
			if !ok {
				fmt.Fprintln(w, "(nil)")
			} else {
				fmt.Fprintln(w, val)
			}
		case "set":
			if len(args) != 2 {
				fmt.Fprintln(w, "ERR SET requires two arguments")
				continue
			}

			key := args[0]
			value := args[1]
			if err := s.Set(key, value); err != nil {
				fmt.Fprintf(w, "ERR SET ko: %v\n", err)
				continue
			}

			fmt.Fprintln(w, "OK")
		case "del":
			if len(args) != 1 {
				fmt.Fprintln(w, "ERR DEL requires one argument")
				continue
			}
			key := args[0]
			ok, err := s.Del(key)
			if err != nil {
				fmt.Fprintf(w, "ERR DEL ko: %v\n", err)
				continue
			}
			if ok {
				fmt.Fprintln(w, "1")
			} else {
				fmt.Fprintln(w, "0")
			}
		case "exit":
			return nil
		default:
			fmt.Fprintf(w, "ERR %s command not found\n", cmd)
		}
	}

	return sc.Err()
}

func parseCmd(input string) (string, []string, error) {
	f := strings.Fields(input)
	if len(f) == 0 {
		return "", nil, nil
	}
	if len(f) == 1 {
		return f[0], nil, nil
	}

	command := f[0]
	_, args, _ := strings.Cut(input, command)

	if strings.Count(args, "'")%2 != 0 {
		return command, nil, errors.New("unbalanced quote number")
	}

	argList := strings.Split(args, "'")
	var argResult []string
	for i, element := range argList {
		if i%2 == 0 {
			argResult = append(argResult, strings.Fields(element)...)
		} else {
			argResult = append(argResult, element)
		}
	}

	return command, argResult, nil
}
