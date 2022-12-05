package shellspy_test

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/mr-joshcrane/shellspy"
)

func TestCommandFromString_(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		input string
		want  []string
	}{
		"converts string into executable cmd": {
			input: "ls -l",
			want:  []string{"ls", "-l"},
		},
		"with no args converts to executable cmd": {
			input: "echo",
			want:  []string{"echo"},
		},
		"does not split quoted arguments": {
			input: "cat 'folder/my file'",
			want:  []string{"cat", "folder/my file"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cmd, err := shellspy.CommandFromString(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			got := cmd.Args
			if !cmp.Equal(tc.want, got) {
				t.Fatal(cmp.Diff(tc.want, got))
			}
		})
	}
}

func TestCommandFromString_WithEmptyStringReturnsError(t *testing.T) {
	t.Parallel()
	_, err := shellspy.CommandFromString("")
	if err == nil {
		t.Fatal(err)
	}
}

func TestSpySession_DoesntTerminateWithoutTerminationSignal(t *testing.T) {
	t.Parallel()
	input := &bytes.Buffer{}
	loops := make(chan bool)
	go func() {
		shellspy.SpySession(input, io.Discard)
		loops <- false
	}()

	go func() {
		time.Sleep(3 * time.Second)
		loops <- true
	}()

	if !<-loops {
		t.Fatal("SpySession terminated without termination signal")
	}
}
