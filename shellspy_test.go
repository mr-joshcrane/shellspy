package shellspy_test

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"syscall"
	"testing"
	"testing/iotest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/mr-joshcrane/shellspy"
	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"local":  shellspy.LocalInstance,
		"server": shellspy.ServerInstance,
	}))
}

func TestLocalInstance(t *testing.T) {
	t.Parallel()
	testscript.Run(t, testscript.Params{Dir: "./testdata/local"})
}

func TestServerInstance(t *testing.T) {
	t.Parallel()
	testscript.Run(t, testscript.Params{Dir: "./testdata/server"})
}
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
	shellspy.NewSpySession(shellspy.WithInput(input), shellspy.WithOutput(&bytes.Buffer{})).Start()
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
	shellspy.NewSpySession(shellspy.WithInput(input), shellspy.WithOutput(buf)).Start()
	want := "$ one\n$ two\n$ three\n$ "
	got := buf.String()
	if !strings.Contains(got, want) {
		t.Fatalf("wanted %q, got %q", want, got)
	}
}

func TestSpySession_PrintsErrorsForFailedCommands(t *testing.T) {
	t.Parallel()
	input := strings.NewReader("nonexistent command\n")
	buf := &bytes.Buffer{}
	shellspy.NewSpySession(shellspy.WithInput(input), shellspy.WithOutput(buf)).Start()
	want := "$ exec: \"nonexistent\": executable file not found in $PATH\n$ "
	got := buf.String()
	if !strings.Contains(got, want) {
		t.Fatalf("wanted %q, got %q", want, got)
	}
}

func TestSpySession_PrintsErrorsForInvalidCommands(t *testing.T) {
	input := strings.NewReader("'''\n\n")
	buf := &bytes.Buffer{}
	shellspy.NewSpySession(shellspy.WithInput(input), shellspy.WithOutput(buf)).Start()
	want := "$ unbalanced quotes or backslashes in [''']\n$ \n$ "
	got := buf.String()
	if !strings.Contains(got, want) {
		t.Fatalf("wanted %q, got %q", want, got)
	}
}

func TestSpySession_ProducesTranscriptOfSession(t *testing.T) {
	t.Parallel()
	input := strings.NewReader("echo one\necho two\necho three\n")
	buf := &bytes.Buffer{}
	session := shellspy.NewSpySession(shellspy.WithInput(input), shellspy.WithOutput(&bytes.Buffer{}), shellspy.WithTranscript(buf))
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
	addr := setupRemoteServer(t, "password", io.Discard)
	conn := setupConnection(t, addr)
	supplyPassword(t, conn, "password")
	time.Sleep(100 * time.Millisecond)
	writeLine(t, conn, "exit")
	err := waitForBrokenPipe(conn)
	if !errors.Is(err, syscall.EPIPE) {
		t.Fatalf("expected broken pipe, but got %v", err)
	}
}

func TestRemoteShell_DisplaysWelcomeOnConnectAndGoodbyeMessageOnExit(t *testing.T) {
	t.Parallel()
	input := strings.NewReader("exit\n")
	buf := &bytes.Buffer{}
	shellspy.NewSpySession(shellspy.WithInput(input), shellspy.WithOutput(buf)).Start()
	want := "Welcome to the remote shell!\n$ Goodbye!\n"
	got := buf.String()
	if want != got {
		t.Fatalf(cmp.Diff(want, got))
	}
}

func TestRemoteShell_AuthClosesSessionOnBadRead(t *testing.T) {
	t.Parallel()
	addr := setupRemoteServer(t, "correctPassword", io.Discard)
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
func TestRemoteShell_AuthKeepsSessionAliveOnCorrectPassword(t *testing.T) {
	t.Parallel()
	addr := setupRemoteServer(t, "correctPassword", io.Discard)
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

type ErrConn struct {
	io.Reader
}

func (e ErrConn) Write(p []byte) (n int, err error) {
	return 0, nil
}

func TestAuthIsFalseForErrorOnRead(t *testing.T) {
	t.Parallel()
	conn := ErrConn{iotest.ErrReader(errors.New("faulty read"))}
	s := shellspy.Server{
		Password: "correctPassword",
		Logger:   io.Discard,
	}
	if s.Auth(conn) {
		t.Fatal(true)
	}
}
func TestAuthIsFalseForIncorrectPassword(t *testing.T) {
	t.Parallel()
	conn := &bytes.Buffer{}
	fmt.Fprintln(conn, "incorrectPassword")
	s := shellspy.Server{
		Password: "correctPassword",
		Logger:   io.Discard,
	}
	if s.Auth(conn) {
		t.Fatal(true)
	}
}

func TestAuthIsTrueForCorrectPassword(t *testing.T) {
	t.Parallel()
	conn := &bytes.Buffer{}
	fmt.Fprintln(conn, "correctPassword")
	s := shellspy.Server{
		Password: "correctPassword",
		Logger:   io.Discard,
	}
	if !s.Auth(conn) {
		t.Fatal(false)
	}
}

func TestServerSideLogging(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	addr := setupRemoteServer(t, "correctPassword", buf)
	c1 := setupConnection(t, addr)
	c2 := setupConnection(t, addr)
	c3 := setupConnection(t, addr)

	fmt.Fprintln(c1, "correctPassword")
	fmt.Fprintln(c2, "incorrectPassword")
	fmt.Fprintln(c3, "correctPassword")

	time.Sleep(1 * time.Second)
	got := strings.Split(buf.String(), "\n")
	got = got[0 : len(got)-1]

	want := []string{
		fmt.Sprintf("Starting listener on %s", addr),
		"Listener created.",
		fmt.Sprintf("Accepting connection from %s", c1.LocalAddr()),
		fmt.Sprintf("Accepting connection from %s", c2.LocalAddr()),
		fmt.Sprintf("Accepting connection from %s", c3.LocalAddr()),
		fmt.Sprintf("SUCCESSFUL LOGIN from %s", c1.LocalAddr()),
		fmt.Sprintf("SUCCESSFUL LOGIN from %s", c3.LocalAddr()),
		fmt.Sprintf("FAILED LOGIN from %s", c2.LocalAddr()),
	}
	less := func(a, b string) bool { return a < b }
	if !cmp.Equal(want, got, cmpopts.SortSlices(less)) {
		t.Fatalf(cmp.Diff(want, got))
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

func writeLine(t *testing.T, conn net.Conn, line string) {
	t.Helper()
	_, err := fmt.Fprintf(conn, line+"\n")
	if err != nil {
		t.Fatalf("attempted to write %s but got err %q", line, err)
	}
}

func waitForBrokenPipe(conn net.Conn) error {
	var err error
	for i := 0; i < 10; i++ {
		_, err = fmt.Fprintf(conn, "echo ':Is pipe broken?'\n")
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

func setupRemoteServer(t *testing.T, password string, logger io.Writer) string {
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
		s := shellspy.Server{
			Address:  addr,
			Password: password,
			Logger:   logger,
		}
		err := s.ListenAndServe()
		if err != nil {
			panic(err)
		}
		if err == nil {
			panic("expected server to block, but it exited with no error")
		}
	}()
	return addr
}

func ExampleServer_Log() {
	s := shellspy.Server{
		Logger: os.Stdout,
	}
	s.Log("Log simple server messages like this")
	// Output:
	// Log simple server messages like this
}
func ExampleServer_Logf() {
	s := shellspy.Server{
		Logger: os.Stdout,
	}
	err := errors.New("a complex message")
	s.Logf("Log %s like this", err)
	// Output:
	// Log a complex message like this
}

func ExampleServer_Start() {

}
