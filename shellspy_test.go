package shellspy_test

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
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

func TestCommandFromString_WithEmptyStringReturnsNeitherCmdNorError(t *testing.T) {
	t.Parallel()
	cmd, err := shellspy.CommandFromString("")
	if cmd != nil {
		t.Fatalf("command was %v", cmd)
	}
	if err != nil {
		t.Fatalf("error was %v", err)
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
	want := "$ unbalanced quotes or backslashes in [''']\n$ "
	got := buf.String()
	if !strings.Contains(got, want) {
		t.Fatalf("want %q should be substring of got %q", want, got)
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
$ `
	got := buf.String()
	if want != got {
		t.Fatal(cmp.Diff(want, got))
	}
}
func TestSpySession_TerminatesOnExitCommand(t *testing.T) {
	t.Parallel()
	s := setupRemoteServer(t, "password", io.Discard)
	conn := setupConnection(t, s.Address)
	supplyPassword(t, conn, "password")
	time.Sleep(100 * time.Millisecond)
	writeLine(t, conn, "exit")
	err := waitForBrokenPipe(conn)
	if !errors.Is(err, syscall.EPIPE) {
		t.Fatalf("expected broken pipe, but got %v", err)
	}
}

func TestRemoteShell_DisplaysTerminalPrompt(t *testing.T) {
	t.Parallel()
	input := strings.NewReader("exit\n")
	buf := &bytes.Buffer{}
	shellspy.NewSpySession(shellspy.WithInput(input), shellspy.WithOutput(buf), shellspy.WithTranscript(io.Discard)).Start()
	want := "$ "
	got := buf.String()
	if want != got {
		t.Fatalf(cmp.Diff(want, got))
	}
}

func TestRemoteShell_AuthClosesSessionOnWrongPassword(t *testing.T) {
	t.Parallel()
	s := setupRemoteServer(t, "correctPassword", io.Discard)
	conn := setupConnection(t, s.Address)
	line := readLine(t, conn)
	if line != "Enter Password: " {
		t.Fatalf("wanted 'Enter Password: ', got %s", line)
	}
	writeLine(t, conn, "wrongPassword")
	line = readLine(t, conn)
	if line != "Incorrect Password: Closing connection" {
		t.Fatalf("wanted 'Incorrect Password: Closing connection', got %s", line)
	}
	err := waitForBrokenPipe(conn)
	if !errors.Is(err, syscall.EPIPE) {
		t.Fatalf("expected error, but got %q", err)
	}
}
func TestRemoteShell_AuthKeepsSessionAliveOnCorrectPassword(t *testing.T) {
	t.Parallel()
	s := setupRemoteServer(t, "correctPassword", io.Discard)
	conn := setupConnection(t, s.Address)
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

func TestAuthIsFalseForErrorOnRead(t *testing.T) {
	t.Parallel()
	conn := ErrConn{iotest.ErrReader(errors.New("faulty read"))}
	s := setupRemoteServer(t, "correctPassword", io.Discard)
	if s.Auth(conn) {
		t.Fatal(true)
	}
}
func TestAuthIsFalseForIncorrectPassword(t *testing.T) {
	t.Parallel()
	conn := &bytes.Buffer{}
	fmt.Fprintln(conn, "incorrectPassword")
	s := setupRemoteServer(t, "correctPassword", io.Discard)
	if s.Auth(conn) {
		t.Fatal(true)
	}
}

func TestAuthIsTrueForCorrectPassword(t *testing.T) {
	t.Parallel()
	conn := &bytes.Buffer{}
	fmt.Fprintln(conn, "correctPassword")
	s := setupRemoteServer(t, "correctPassword", io.Discard)
	if !s.Auth(conn) {
		t.Fatal(false)
	}
}

func TestServerSideLogging(t *testing.T) {
	buf := &bytes.Buffer{}
	s := setupRemoteServer(t, "correctPassword", buf)
	c1 := setupConnection(t, s.Address)
	c2 := setupConnection(t, s.Address)
	c3 := setupConnection(t, s.Address)

	fmt.Fprintln(c1, "correctPassword")
	fmt.Fprintln(c2, "incorrectPassword")
	fmt.Fprintln(c3, "correctPassword")

	time.Sleep(50 * time.Millisecond)
	got := strings.Split(buf.String(), "\n")
	got = got[0 : len(got)-1]

	want := []string{
		fmt.Sprintf("Starting listener on %s", s.Address),
		"Listener created.",
		fmt.Sprintf("Accepting connection from %s", c1.LocalAddr()),
		fmt.Sprintf("Accepting connection from %s", c2.LocalAddr()),
		fmt.Sprintf("Accepting connection from %s", c3.LocalAddr()),
		fmt.Sprintf("SUCCESSFUL LOGIN from %s", c1.LocalAddr()),
		fmt.Sprintf("SUCCESSFUL LOGIN from %s", c3.LocalAddr()),
		fmt.Sprintf("FAILED LOGIN from %s", c2.LocalAddr()),
		fmt.Sprintf("Transcript for new session available at %s/transcript-%d.txt", s.TranscriptDirectory, 1),
		fmt.Sprintf("Transcript for new session available at %s/transcript-%d.txt", s.TranscriptDirectory, 2),
	}
	less := func(a, b string) bool { return a < b }
	if !cmp.Equal(want, got, cmpopts.SortSlices(less)) {
		t.Fatalf(cmp.Diff(want, got))
	}
}

func TestServerSideTranscriptsAreOnePerSuccessfulConnection(t *testing.T) {
	s := setupRemoteServer(t, "correctPassword", io.Discard)

	c1 := setupConnection(t, s.Address)
	fmt.Fprintln(c1, "correctPassword")

	c2 := setupConnection(t, s.Address)
	fmt.Fprintln(c2, "incorrectPassword")

	c3 := setupConnection(t, s.Address)
	fmt.Fprintln(c3, "correctPassword")

	time.Sleep(50 * time.Millisecond)
	got := numberOfFilesInFolder(s.TranscriptDirectory)

	if got != 2 {
		t.Fatalf("expected 2 files in transcript folder but got %d", got)
	}
}

func TestServerLogsErrorWhenTranscriptUnavailable(t *testing.T) {
	buf := &bytes.Buffer{}

	s := setupRemoteServer(t, "correctPassword", buf)
	os.Chmod(s.TranscriptDirectory, 0o444)
	c1 := setupConnection(t, s.Address)

	fmt.Fprintln(c1, "correctPassword")

	time.Sleep(50 * time.Millisecond)
	got := strings.Split(buf.String(), "\n")
	got = got[0 : len(got)-1]

	want := []string{
		fmt.Sprintf("Starting listener on %s", s.Address),
		"Listener created.",
		fmt.Sprintf("Accepting connection from %s", c1.LocalAddr()),
		fmt.Sprintf("SUCCESSFUL LOGIN from %s", c1.LocalAddr()),
		fmt.Sprintf("open %s/transcript-%d.txt: permission denied", s.TranscriptDirectory, 1),
	}
	less := func(a, b string) bool { return a < b }
	if !cmp.Equal(want, got, cmpopts.SortSlices(less)) {
		t.Fatalf(cmp.Diff(want, got))
	}
}

func ExampleServer_Log() {
	s := shellspy.NewServer("serverAddress", "password", "logDirectory")
	s.Log("Log simple server messages like this")
	// Output:
	// Log simple server messages like this
}
func ExampleServer_Logf() {
	s := shellspy.NewServer("serverAddress", "password", "logDirectory")
	err := errors.New("a complex message")
	s.Logf("Log %s like this", err)
	// Output:
	// Log a complex message like this
}

func ExampleWithInput() {
	shellspy.NewSpySession(shellspy.WithInput(os.Stdin))
}

func ExampleWithOutput() {
	shellspy.NewSpySession(shellspy.WithOutput(os.Stdout))
}

func ExampleWithTranscript() {
	// To see the transcript in the terminal, you might want to use [os.Stdout]
	shellspy.NewSpySession(shellspy.WithTranscript(os.Stdout))

	// Alternatively you might want to direct it to an [io.Writer]
	buf := bytes.NewBuffer(nil)
	shellspy.NewSpySession(shellspy.WithTranscript(buf))
}

func ExampleWithConnection() {
	conn, _ := net.Dial("tcp", "localhost:8080")
	shellspy.NewSpySession(shellspy.WithConnection(conn))
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

func getFreeListenerAddress(t *testing.T) (addr string, err error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	err = listener.Close()
	if err != nil {
		return "", err
	}
	for i := 0; i < 10; i++ {
		_, err := net.Dial("tcp", listener.Addr().String())
		if err != nil {
			return listener.Addr().String(), nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return "", fmt.Errorf("listener did not close in a timely fashion")
}

func setupRemoteServer(t *testing.T, password string, logger io.Writer) *shellspy.Server {
	t.Helper()

	addr, err := getFreeListenerAddress(t)
	if err != nil {
		t.Fatal(err)
	}
	tempDir := t.TempDir()
	s := shellspy.NewServer(addr, password, tempDir)
	s.Logger = logger
	go func() {
		err := s.ListenAndServe()
		if err != nil {
			panic(err)
		}
		if err == nil {
			panic("expected server to block, but it exited with no error")
		}
	}()
	return s
}

func numberOfFilesInFolder(path string) int {
	folder, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer folder.Close()
	files, err := folder.ReadDir(-1)
	if err != nil {
		log.Fatal(err)
	}
	return len(files)
}

type ErrConn struct {
	io.Reader
}

func (e ErrConn) Write(p []byte) (n int, err error) {
	return 0, nil
}

func FuzzXxx(f *testing.F) {
	f.Fuzz(func(t *testing.T, input string) {
		buf := &bytes.Buffer{}
		reader, writer := io.Pipe()
		s := shellspy.NewSpySession(shellspy.WithInput(reader), shellspy.WithTranscript(buf), shellspy.WithOutput(os.Stdout))
		go s.Start()
		fmt.Fprintln(writer, input)

	})
}
