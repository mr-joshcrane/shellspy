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

func NewServer(password string) *Server {
	return &Server{
		Logger:   os.Stderr,
		Password: password,
	}
}

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

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	if !s.Auth(conn) {
		s.Logf("FAILED LOGIN from %s\n", conn.RemoteAddr())
		return
	}
	s.Logf("SUCCESSFUL LOGIN from %s\n", conn.RemoteAddr())

	session := SpySession(conn, conn)
	err := session.Start()
	if err != nil {
		s.Log(err)
		return
	}
}

func (s *Server) Log(args ...any) {
	fmt.Fprintln(s.Logger, args...)
}

func (s *Server) Logf(str string, args ...any) {
	fmt.Fprintf(s.Logger, str, args...)
}

type Session struct {
	r          io.Reader
	output     io.Writer
	Transcript io.Writer
	Closed     bool
}

func SpySession(r io.Reader, w io.Writer) Session {
	return Session{
		r:          r,
		output:     w,
		Transcript: io.Discard,
		Closed:     false,
	}
}

func NewSpySession() Session {
	return SpySession(os.Stdin, os.Stdout)
}

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

func ListenAndServe(addr string, serverPassword string) error {
	s := NewServer(serverPassword)
	return s.ListenAndServe()
}

// create a server struct
// move the auth method to server
// move the welcome messages inside Start
// we have to call "handle" from Listen and serve
