package shellspy

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"

	"bitbucket.org/creachadair/shell"
)

// CommandFromString takes a string and converts it into a
// pointer to a exec.Cmd struct. It will return an error if
// there are unbalanced quotes or backslashes in the string.
func CommandFromString(s string) (*exec.Cmd, error) {
	commands, ok := shell.Split(s)
	if !ok {
		return nil, fmt.Errorf("unbalanced quotes or backslashes in [%s]", s)
	}
	if len(commands) == 0 {
		return nil, fmt.Errorf("")
	}
	path := commands[0]
	args := commands[1:]
	return exec.Command(path, args...), nil
}

type Server struct {
	Address  string
	Password string
	Logger   io.Writer
}

// Convenience wrapper for the Server struct with sensible defaults.
func NewServer(password string) *Server {
	return &Server{
		Logger:   os.Stderr,
		Password: password,
	}
}

// Convenience method on Servers to set the address and start the server.
func (s *Server) ListenAndServeOn(addr string) error {
	s.Address = addr
	return s.ListenAndServe()
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
			return fmt.Errorf("Connection error: %w", err)
		}
		s.Logf("Accepting connection from %s\n", conn.RemoteAddr())
		go s.handle(conn)
	}
}

// Auth is a method on server that takes a connection object,
// challenges the user for the server password, returning true
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

// handle is a method on server that takes a connection,
// Provides an auth challenge, and initiates a session on
// successful login. Will not create a session on failed auth challenge.
func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	if !s.Auth(conn) {
		s.Logf("FAILED LOGIN from %s\n", conn.RemoteAddr())
		return
	}
	s.Logf("SUCCESSFUL LOGIN from %s\n", conn.RemoteAddr())

	session := NewSpySession(WithConnection(conn))
	err := session.Start()
	if err != nil {
		s.Log(err)
		return
	}
}

// Log is a convenience method for server that wraps Fprintln.
func (s *Server) Log(args ...any) {
	fmt.Fprintln(s.Logger, args...)
}

// Logf is a convenience method for server that wraps Fprintf.
func (s *Server) Logf(str string, args ...any) {
	fmt.Fprintf(s.Logger, str, args...)
}

type Session struct {
	r          io.Reader
	output     io.Writer
	Transcript io.Writer
	Closed     bool
}

type SessionOption func(*Session) *Session

func WithInput(input io.Reader) SessionOption {
	return func(s *Session) *Session {
		s.r = input
		return s
	}
}
func WithOutput(output io.Writer) SessionOption {
	return func(s *Session) *Session {
		s.output = output
		return s
	}
}

func WithTranscript(transcript io.Writer) SessionOption {
	return func(s *Session) *Session {
		s.Transcript = transcript
		return s
	}
}

func WithConnection(conn net.Conn) SessionOption {
	return func(s *Session) *Session {
		s.r = conn
		s.output = conn
		return s
	}
}

// Convenience wrapped around Session with default arguments.
func NewSpySession(opts ...SessionOption) *Session {
	s := &Session{
		r:          os.Stdin,
		output:     os.Stdout,
		Transcript: io.Discard,
		Closed:     false,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Start is a method on Session that takes no arguments and
// returns an error. It will read from the session's input
// and write to the session's output. It will (optionally)
// write to the sessions transcript.
func (s Session) Start() error {
	fmt.Fprintln(s.output, "Welcome to the remote shell!")
	w := io.MultiWriter(s.output, s.Transcript)
	fmt.Fprint(s.output, "$ ")
	scan := bufio.NewScanner(s.r)

	for scan.Scan() {
		line := scan.Text()
		if line == "exit" {
			fmt.Fprintf(s.Transcript, "exit\n")
			break
		}
		fmt.Fprintf(s.Transcript, "$ %s\n", line)
		cmd, err := CommandFromString(line)
		if err != nil {
			fmt.Fprintln(w, err)
			fmt.Fprint(w, "$ ")
			continue
		}
		cmd.Stdout = w
		cmd.Stderr = w
		err = cmd.Run()
		if err != nil {
			fmt.Fprintln(w, err)
		}
		fmt.Fprint(s.output, "$ ")
	}
	fmt.Fprintln(s.output, "Goodbye!")
	return scan.Err()
}

// ListenAndServe is a convenience wrapper that starts a
// blocking server that is listening on the supplied port.
func ListenAndServe(addr string, serverPassword string) error {
	s := NewServer(serverPassword)
	return s.ListenAndServe()
}

func LocalInstance() int {
	newFile, err := os.Create("transcript.txt")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	session := NewSpySession()
	session.Transcript = newFile
	session.Start()
	return 0
}

func ServerInstance() int {
	PORT := os.Getenv("PORT")
	if PORT == "" {
		fmt.Println("PORT environment variable must be set")
		os.Exit(1)
	}
	PASSWORD := os.Getenv("PASSWORD")
	if PASSWORD == "" {
		fmt.Println("PASSWORD environment variable must be set")
		os.Exit(1)
	}

	fmt.Println("Starting shellspy on port", PORT)
	err := ListenAndServe(fmt.Sprintf("0.0.0.0:%s", PORT), PASSWORD)
	panic(err)
}
