package repl

import (
	"bufio"
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/ivanpasq/tana/internal/store"
)

func TestParseCmd(t *testing.T) {
	testCases := []struct {
		name     string
		in       string
		wantCmd  string
		wantArgs []string
		wantErr  bool
	}{
		{
			name:    "empty string",
			in:      "",
			wantCmd: "",
		},
		{
			name:    "only whitespaces",
			in:      "   ",
			wantCmd: "",
		},
		{
			name:    "just command",
			in:      "GET",
			wantCmd: "GET",
		},
		{
			name:     "spaces before command",
			in:       "  GET a",
			wantCmd:  "GET",
			wantArgs: []string{"a"},
		},
		{
			name:     "simple arguments",
			in:       "SET a b",
			wantCmd:  "SET",
			wantArgs: []string{"a", "b"},
		},
		{
			name:     "value within quotes with spaces",
			in:       "SET a 'ciao   mondo'",
			wantCmd:  "SET",
			wantArgs: []string{"a", "ciao   mondo"},
		},
		{
			name:     "args key between quotes",
			in:       "SET 'k k' v",
			wantCmd:  "SET",
			wantArgs: []string{"k k", "v"},
		},
		{
			name:     "empty quotes content",
			in:       "SET a ''",
			wantCmd:  "SET",
			wantArgs: []string{"a", ""},
		},
		{
			name:     "spaces within quote boundaries",
			in:       "SET a '  x  '",
			wantCmd:  "SET",
			wantArgs: []string{"a", "  x  "},
		},
		{
			name:     "mixed spaces and quotes",
			in:       "SET a b 'c'",
			wantCmd:  "SET",
			wantArgs: []string{"a", "b", "c"},
		},
		{
			name:    "odd quotes number",
			in:      "SET a 'b",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotCmd, gotArgs, err := parseCmd(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error got %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if gotCmd != tc.wantCmd {
				t.Errorf("command: got %q, want %q", gotCmd, tc.wantCmd)
			}
			if !slices.Equal(gotArgs, tc.wantArgs) {
				t.Errorf("args: got %q, want %q", gotArgs, tc.wantArgs)
			}
		})
	}
}

func TestRepl(t *testing.T) {
	testCases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{
			name: "set and get",
			in:   "SET a 1\nGET a\n",
			want: "OK\n1\n",
		},
		{
			name: "key not existent",
			in:   "GET a\n",
			want: "(nil)\n",
		},
		{
			name: "double delete",
			in:   "SET a 1\nDEL a\nDEL a\nGET a\n",
			want: "OK\n1\n0\n(nil)\n",
		},
		{
			name: "case-insensitive",
			in:   "set a 1\nGeT a\n",
			want: "OK\n1\n",
		},
		{
			name: "case-sensitive key",
			in:   "SET a 1\nGET A\n",
			want: "OK\n(nil)\n",
		},
		{
			name: "quoted value",
			in:   "SET a 'ciao   mondo'\nGET a\n",
			want: "OK\nciao   mondo\n",
		},
		{
			name: "wrong arguments",
			in:   "GET\nSET a\nDEL a b\n",
			want: "ERR GET requires one argument\nERR SET requires two arguments\nERR DEL requires one argument\n",
		},
		{
			name: "unknown command",
			in:   "FOO a\n",
			want: "ERR FOO command not found\n",
		},
		{
			name: "odd quotes number",
			in:   "SET a 'b\n",
			want: "ERR unbalanced quote number\n",
		},
		{
			name: "ignore empty line",
			in:   "\n   \nGET a\n",
			want: "(nil)\n",
		},
		{
			name: "exit stops repl",
			in:   "SET a 1\nEXIT\nGET a\n",
			want: "OK\n",
		},
		{
			name: "override",
			in:   "SET a 1\nSET a 2\nGET a\n",
			want: "OK\nOK\n2\n",
		},
		{
			name:    "input too long",
			in:      strings.Repeat("x", bufio.MaxScanTokenSize+1) + "\n",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := store.New()
			r := strings.NewReader(tc.in)
			var buf bytes.Buffer

			err := Run(s, r, &buf)

			if tc.wantErr != (err != nil) {
				t.Fatalf("error: got %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if tc.want != buf.String() {
				t.Errorf("repl result: got %q, want %q", buf.String(), tc.want)
			}
		})
	}
}
