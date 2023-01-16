package shellspy_test

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"
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

func setupRemoteServer(t *testing.T, password string) string {
	t.Helper()
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
		err := shellspy.ListenAndServe(addr, shellspy.NewPassword("password"))
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
	addr := setupRemoteServer(t, "password")
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fmt.Fprintln(conn, "password")
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
	want := []string{"Enter Password: ", "Welcome to the remote shell!", "$ exit", "Goodbye!"}
	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}

func TestRemoteShell_AuthClosesSessionOnIncorrectPasswordOld(t *testing.T) {
	t.Parallel()
	serverR, clientW := io.Pipe()
	clientR, serverW := io.Pipe()
	session := shellspy.SpySession(serverR, serverW)
	scan := bufio.NewScanner(clientR)
	authenticated := make(chan bool)
	go func() { authenticated <- session.Auth(shellspy.NewPassword("password")) }()
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

func setupConnection(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	for err != nil {
		time.Sleep(50 * time.Millisecond)
		conn, err = net.Dial("tcp", addr)
	}
	return conn
}

func readLine(t *testing.T, conn net.Conn) string {
	t.Helper()
	scan := bufio.NewScanner(conn)
	if !scan.Scan() {
		t.Fatalf("nothing to read from conn, scan.Err says %q", scan.Err())
	}
	return scan.Text()
}

func writeLine(t *testing.T, conn net.Conn, password string) {
	t.Helper()
	_, err := fmt.Fprintf(conn, password+"\n")
	if err != nil {
		t.Fatalf("attempted to write %s but got err %q", password, err)
	}
}

func waitForBrokenPipe(conn net.Conn) error {
	var err error
	for i := 0; i < 3; i++ {
		_, err = fmt.Fprintf(conn, "echo 'Is pipe broken?'\n")
		time.Sleep(50 * time.Millisecond)
	}
	return err
}

func TestRemoteShell_AuthClosesSessionOnIncorrectPassword(t *testing.T) {
	t.Parallel()
	addr := setupRemoteServer(t, "correctPassword")
	conn := setupConnection(t, addr)
	line := readLine(t, conn)
	if line != "Enter Password: " {
		t.Fatalf("wanted 'Enter Password: ', got %s", line)
	}
	writeLine(t, conn, "incorrectPassword")
	readLine(t, conn)
	err := waitForBrokenPipe(conn)
	if !errors.Is(err, syscall.EPIPE) {
		t.Fatalf("expected a broken pipe, but got %q", err)
	}

}

func TestRemoteShell_AuthKeepsSessionAliveOnCorrectPassword(t *testing.T) {
	t.Parallel()
	serverR, clientW := io.Pipe()
	clientR, serverW := io.Pipe()
	session := shellspy.SpySession(serverR, serverW)
	scan := bufio.NewScanner(clientR)
	authenticated := make(chan bool)
	go func() { authenticated <- session.Auth(shellspy.NewPassword("password")) }()
	go func() { time.Sleep(3 * time.Second); panic("Timed out!") }()
	for scan.Scan() {
		prompt := scan.Text()
		want := "Enter Password: "
		if !cmp.Equal(prompt, want) {
			t.Fatalf(cmp.Diff(prompt, want))
		}
		break
	}
	fmt.Fprintln(clientW, "password")
	if !<-authenticated {
		t.Fatal("Should be authenticated!")
	}
	if session.Closed {
		t.Fatal("Session should be open!")
	}
}
