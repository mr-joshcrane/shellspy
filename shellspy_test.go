package shellspy_test

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
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

func TestSpySession_ReadsUserInputToCompletion(t *testing.T) {
	t.Parallel()
	input := strings.NewReader("test input one\ntest input two\ntest input three\n")
	shellspy.SpySession(input, io.Discard).Start()
	contents, err := io.ReadAll(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) != 0 {
		t.Fatalf("didn't expect any content left in buffer, but got %q", contents)
	}
}

func TestSpySession_ExecutesCommandsAndOutputsResults(t *testing.T) {
	t.Parallel()
	input := strings.NewReader("echo one\necho two\necho three\n")
	buf := &bytes.Buffer{}
	shellspy.SpySession(input, buf).Start()
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
	shellspy.SpySession(input, buf).Start()
	want := "$ exec: \"nonexistent\": executable file not found in $PATH\n$ "
	got := buf.String()
	if want != got {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestSpySession_PrintsErrorsForInvalidCommands(t *testing.T) {
	input := strings.NewReader("'''\n\n")
	buf := &bytes.Buffer{}
	shellspy.SpySession(input, buf).Start()
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
	session := shellspy.SpySession(input, io.Discard)
	session.Transcript = buf
	session.Start()
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

func setupRemoteServer(t *testing.T) string {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	err = listener.Close()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	addr := listener.Addr().String()
	go func() {
		err := shellspy.ListenAndServe(addr)
		if err != nil {
			panic(err)
		}
		if err == nil {
			panic("expected server to block, but it exited with no error")
		}
	}()
	return addr
}

func TestSpySession_TerminatesOnExitCommand(t *testing.T) {
	t.Parallel()
	r, w := io.Pipe()
	session := shellspy.SpySession(r, io.Discard)
	errChan := make(chan error)

	go func() { errChan <- session.Start() }()
	go func() { time.Sleep(time.Second); errChan <- errors.New("Session timed out") }()
	fmt.Fprintln(w, "exit")
	done := <-errChan
	if done != io.EOF {
		t.Fatal("Expected session to be done but was not")
	}
}

func TestRemoteShell_DisplaysWelcomeOnConnectAndGoodbyeMessageOnExit(t *testing.T) {
	t.Parallel()
	addr := setupRemoteServer(t)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	conn.SetDeadline(deadline)
	got := []string{}
	_, err = fmt.Fprintln(conn, "exit")
	if err != nil {
		t.Fatal(err)
	}
	scan := bufio.NewScanner(conn)
	for scan.Scan() {
		got = append(got, scan.Text())
	}
	want := []string{"Welcome to the remote shell!", "$ exit", "Goodbye!"}
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestRemoteShell_ClosesSessionOnIncorrectPassword(t *testing.T) {
	t.Parallel()
	serverR, clientW := io.Pipe()
	clientR, serverW := io.Pipe()
	session := shellspy.SpySession(serverR, serverW)
	scan := bufio.NewScanner(clientR)
	authenticated := make(chan bool)
	go func() { authenticated <- session.Auth() }()
	go func() { time.Sleep(3 * time.Second); panic("Timed out!") }()
	for scan.Scan() {
		prompt := scan.Text()
		want := "Enter Password: "
		if !cmp.Equal(prompt, want) {
			t.Fatalf(cmp.Diff(prompt, want))
		}
		break
	}
	fmt.Fprintln(clientW, "wrongpassword")
	for scan.Scan() {
		prompt := scan.Text()
		want := "Incorrect Password: Closing connection"
		if !cmp.Equal(prompt, want) {
			t.Fatal(cmp.Diff(prompt, want))
		}
		break
	}
	if <-authenticated {
		t.Fatal("Should not be authenticated!")
	}
	if !session.Closed {
		t.Fatal("Session should be closed!")
	}
}
