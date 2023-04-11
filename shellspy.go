package shellspy

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"sync/atomic"

	"bitbucket.org/creachadair/shell"
)

// CommandFromString takes a string and converts it into a
// pointer to a [exec.Cmd] struct. It will return an error if
// there are unbalanced quotes or backslashes in the string.
func CommandFromString(s string) (*exec.Cmd, error) {
	commands, ok := shell.Split(s)
	if !ok {
		return nil, fmt.Errorf("unbalanced quotes or backslashes in [%s]", s)
	}
	if len(commands) == 0 {
		return nil, nil
	}
	path := commands[0]
	args := commands[1:]
	return exec.Command(path, args...), nil
}

type Server struct {
	Address             string
	Password            string
	Logger              io.Writer
	TranscriptDirectory string
	TranscriptCounter   atomic.Uint64
}

// NewServer is a convenience wrapper for the [Server] struct with sensible defaults.
func NewServer(addr, password, transcriptDirectory string) *Server {
	return &Server{
		Logger:              os.Stderr,
		Password:            password,
		Address:             addr,
		TranscriptDirectory: transcriptDirectory,
		TranscriptCounter:   atomic.Uint64{},
	}
}

// ListenAndServe listens on the provided address and starts a goroutine
// for to support multiple simultaneous connections.
func (s *Server) ListenAndServe() error {
	s.Logf("Starting listener on %s\n", s.Address)
	listener, err := net.Listen("tcp", s.Address)
	if err != nil {
		s.Log(err)
		return err
	}
	s.Log("Listener created.")
	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			s.Log(err)
			return fmt.Errorf("connection error: %w", err)
		}
		s.Logf("Accepting connection from %s\n", conn.RemoteAddr())
		go s.handle(conn)
	}
}

// Auth is a method on server that takes a [net.conn],
// challenges the user for the [server] password, returning true
// if the password matches and false if it does not.
func (s *Server) Auth(conn io.ReadWriter) bool {
	fmt.Fprintln(conn, "Enter Password: ")
	scan := bufio.NewScanner(conn)
	if !scan.Scan() {
		s.Log(scan.Err())
		return false
	}
	if scan.Text() == s.Password {
		return true
	}
	fmt.Fprintln(conn, "Incorrect Password: Closing connection")
	return false
}

// handle is a method on server that takes a [net.Conn],
// Provides an [Auth] challenge, and initiates a [session] on
// successful login. Will not create a [session] on failed [Auth] challenge.
func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	if !s.Auth(conn) {
		s.Logf("FAILED LOGIN from %s\n", conn.RemoteAddr())
		return
	}
	s.Logf("SUCCESSFUL LOGIN from %s\n", conn.RemoteAddr())
	transcriptLogName := fmt.Sprint(s.TranscriptCounter.Add(1))
	filename := fmt.Sprintf("%s/transcript-%s.txt", s.TranscriptDirectory, transcriptLogName)
	file, err := os.Create(filename)
	if err != nil {
		s.Log(err)
		//justification
		panic(err)
	}
	s.Logf("Transcript for new session available at %s\n", filename)
	session := NewSpySession(WithConnection(conn), WithTranscript(file))
	err = session.Start()
	if err != nil {
		s.Log(err)
		return
	}
}

// Log writes args to the [Server.Logger].
func (s *Server) Log(args ...any) {
	fmt.Fprintln(s.Logger, args...)
}

// Logf formats args into str and logs via [Server.Logger].
func (s *Server) Logf(str string, args ...any) {
	fmt.Fprintf(s.Logger, str, args...)
}

type session struct {
	input          io.Reader
	terminal       io.Writer
	transcript     io.Writer
	combinedOutput io.Writer
}

type SessionOption func(*session) *session

func WithInput(input io.Reader) SessionOption {
	return func(s *session) *session {
		s.input = input
		return s
	}
}
func WithOutput(output io.Writer) SessionOption {
	return func(s *session) *session {
		s.terminal = output
		return s
	}
}

func WithTranscript(transcript io.Writer) SessionOption {
	return func(s *session) *session {
		s.transcript = transcript
		return s
	}
}

func WithConnection(conn net.Conn) SessionOption {
	return func(s *session) *session {
		s.input = conn
		s.terminal = conn
		return s
	}
}

// Convenience wrapped around Session with default arguments.
func NewSpySession(opts ...SessionOption) *session {
	s := &session{
		input:      os.Stdin,
		terminal:   os.Stdout,
		transcript: io.Discard,
	}
	for _, opt := range opts {
		opt(s)
	}
	s.combinedOutput = io.MultiWriter(s.terminal, s.transcript)
	return s
}

func (s session) printPromptToCombinedOutput() {
	fmt.Fprint(s.combinedOutput, "$ ")
}

func (s session) printMessageToUser(msg string) {
	fmt.Fprintln(s.terminal, msg)
}

// Start reads from the [session] input
// and write to the [session] output. It will also
// write to the [session] transcript.
func (s session) Start() error {
	s.printMessageToUser("Welcome to the remote shell!")
	s.printPromptToCombinedOutput()
	scan := bufio.NewScanner(s.input)
	for scan.Scan() {
		line := scan.Text()
		err := s.processLine(line)
		if err == io.EOF {
			break
		}
	}
	s.printMessageToUser("Goodbye!")
	return scan.Err()
}

func (s *session) processLine(line string) error {
	fmt.Fprintf(s.transcript, "%s\n", line)
	if line == "exit" {
		return io.EOF
	}
	cmd, err := CommandFromString(line)
	if err != nil {
		fmt.Fprintln(s.combinedOutput, err)
		s.printPromptToCombinedOutput()
		return nil
	}
	if cmd == nil {
		return nil
	}
	cmd.Stdout = s.combinedOutput
	cmd.Stderr = s.combinedOutput
	err = cmd.Run()
	if err != nil {
		fmt.Fprintln(s.combinedOutput, err)
	}
	s.printPromptToCombinedOutput()
	return nil
}

// ListenAndServe starts listening on the supplied port.
// It does not return until the server is shutdown.
func ListenAndServe(addr, serverPassword, logDir string) error {
	s := NewServer(addr, serverPassword, logDir)
	return s.ListenAndServe()
}

func LocalInstance() int {
	newFile, err := os.Create("transcript.txt")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer newFile.Close()
	session := NewSpySession(WithTranscript(newFile))
	err = session.Start()
	if err != nil {
		return 1
	}
	fmt.Fprintln(session.terminal, "Transcript saved to transcript.txt")
	return 0
}

var ErrServerClosed = errors.New("Server closed")

func ServerInstance() int {
	PORT := os.Getenv("PORT")
	if PORT == "" {
		fmt.Fprintln(os.Stderr, "PORT environment variable must be set")
		return 1
	}
	PASSWORD := os.Getenv("PASSWORD")
	if PASSWORD == "" {
		fmt.Fprintln(os.Stderr, "PASSWORD environment variable must be set")
		return 1
	}
	LOG_DIR := os.Getenv("LOG_DIR")
	if LOG_DIR == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		LOG_DIR = fmt.Sprintf("%s/transcripts", cwd)
		fmt.Fprintf(os.Stdout, "LOG_DIR environment variable not set, defaulting to %s\n", LOG_DIR)
	}
	err := createDirectoryIfNotExists(LOG_DIR)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println("Starting shellspy on port", PORT)
	if err := ListenAndServe(fmt.Sprintf("0.0.0.0:%s", PORT), PASSWORD, LOG_DIR); err != nil && err != ErrServerClosed {
		fmt.Fprint(os.Stderr, err)
		return 1
	}
	return 0
}

func createDirectoryIfNotExists(path string) error {
	dir, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(path, 0755)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		if !dir.IsDir() {
			return fmt.Errorf("path %s is not a directory", path)
		}
	}
	return nil
}
