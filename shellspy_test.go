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
func TestSpySession_TerminatesOnExitCommand(t *testing.T) {
	t.Parallel()
	addr := setupRemoteServer(t, "password")
	conn := setupConnection(t, addr)
	supplyPassword(t, conn, "password")
	writeLine(t, conn, "exit")
	err := waitForBrokenPipe(conn)
	if !errors.Is(err, syscall.EPIPE) {
		t.Fatalf("expected broken pipe, but got %v", err)
	}
}

func TestRemoteShell_DisplaysWelcomeOnConnectAndGoodbyeMessageOnExit(t *testing.T) {
	t.Parallel()
	addr := setupRemoteServer(t, "password")
	conn := setupConnection(t, addr)
	conn.SetDeadline(time.Now().Add(1 * time.Second))
	supplyPassword(t, conn, "password")
	line := readLine(t, conn)
	if line != "Welcome to the remote shell!" {
		t.Fatalf("expected welcome message, but got %q", line)
	}
	writeLine(t, conn, "exit")
	contents, err := io.ReadAll(conn)
	if err != nil {
		t.Fatal(err)
	}
	want := "Goodbye!\n"
	if !strings.Contains(string(contents), want) {
		t.Fatalf("expected %s message, but got %q", want, contents)
	}
}
func TestRemoteShell_AuthClosesSessionOnIncorrectPassword(t *testing.T) {
	t.Parallel()
	addr := setupRemoteServer(t, "correctPassword")
	conn := setupConnection(t, addr)
	supplyPassword(t, conn, "incorrectPassword")
	err := waitForBrokenPipe(conn)
	if !errors.Is(err, syscall.EPIPE) {
		t.Fatalf("expected a broken pipe, but got %q", err)
	}

}

func TestRemoteShell_AuthKeepsSessionAliveOnCorrectPassword(t *testing.T) {
	t.Parallel()
	addr := setupRemoteServer(t, "correctPassword")
	conn := setupConnection(t, addr)
	line := readLine(t, conn)
	if line != "Enter Password: " {
		t.Fatalf("wanted 'Enter Password: ', got %s", line)
	}
	writeLine(t, conn, "correctPassword")
	line = readLine(t, conn)
	if line != "Welcome to the remote shell!" {
		t.Fatalf("wanted 'Welcome to the remote shell!', got %s", line)
	}
	err := waitForBrokenPipe(conn)
	if err != nil {
		t.Fatalf("expected no error, but got %q", err)
	}
}

func TestRemoteShell_AuthLogsFailedLoginAttempts(t *testing.T) {
	t.Parallel()
	rbuf := &bytes.Buffer{}
	tbuf := &bytes.Buffer{}
	fmt.Fprintln(rbuf, "incorrectPassword")

	session := shellspy.SpySession(rbuf, io.Discard)
	session.Transcript = tbuf
	session.Auth(shellspy.NewPassword("correctPassword\n"))
	got := tbuf.String()
	want := "FAILED LOGIN\n"
	if got != want {
		t.Fatalf("expected %s, got %q", want, got)
	}
}

func TestRemoteShell_AuthLogsSuccessfulLoginAttempts(t *testing.T) {
	t.Parallel()
	rbuf := &bytes.Buffer{}
	tbuf := &bytes.Buffer{}
	fmt.Fprintln(rbuf, "correctPassword")

	session := shellspy.SpySession(rbuf, io.Discard)
	session.Transcript = tbuf
	session.Auth(shellspy.NewPassword("correctPassword"))
	got := tbuf.String()
	want := "SUCCESSFUL LOGIN\n"
	if got != want {
		t.Fatalf("expected %s, got %q", want, got)
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

func supplyPassword(t *testing.T, conn net.Conn, password string) {
	t.Helper()
	line := readLine(t, conn)
	if line != "Enter Password: " {
		t.Fatalf("expected to read 'Enter Password: ' but got %q", line)
	}
	writeLine(t, conn, password)
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
		err := shellspy.ListenAndServe(addr, shellspy.NewPassword(password))
		if err != nil {
			panic(err)
		}
		if err == nil {
			panic("expected server to block, but it exited with no error")
		}
	}()
	return addr
}
