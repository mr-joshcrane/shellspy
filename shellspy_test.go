package shellspy_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

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

func TestSpySession_ReadsUserInputToCompletion(t *testing.T) {
	t.Parallel()
	input := strings.NewReader("test input one\ntest input two\ntest input three\n")
	shellspy.SpySession(input, io.Discard, io.Discard)
	contents, err := io.ReadAll(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) != 0 {
		t.Fatalf("didn't expect any content left in buffer, but got %q", contents)
	}
}

func TestSpySession_(t *testing.T) {
	t.Parallel()
	input := strings.NewReader("echo one\necho two\necho three\n")
	buf := &bytes.Buffer{}
	shellspy.SpySession(input, buf, io.Discard)
	want := "$ one\n$ two\n$ three\n$ "
	got := buf.String()
	if want != got {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestSpySession_PrintsErrorsForFailedCommands(t *testing.T) {
	t.Parallel()
	input := strings.NewReader("nonexistent command\n")
	buf := &bytes.Buffer{}
	shellspy.SpySession(input, buf, io.Discard)
	want := "$ exec: \"nonexistent\": executable file not found in $PATH\n$ "
	got := buf.String()
	if want != got {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestSpySession_PrintsErrorsForInvalidCommands(t *testing.T) {
	input := strings.NewReader("'''\n\n")
	buf := &bytes.Buffer{}
	shellspy.SpySession(input, buf, io.Discard)
	want := "$ unbalanced quotes or backslashes in [''']\n$ \n$ "
	got := buf.String()
	if want != got {
		t.Fatal(cmp.Diff(want, got))
	}

}

func TestSpySession_ProducesTranscriptOfSession(t *testing.T) {
	t.Parallel()
	input := strings.NewReader("echo one\necho two\necho three\n")
	buf := &bytes.Buffer{}
	shellspy.SpySession(input, io.Discard, buf)
	want := `$ echo one
	one
	$ echo two
	two
	$ echo three
	three
	`
	got := buf.String()
	if want != got {
		t.Fatal(cmp.Diff(want, got))
	}
}