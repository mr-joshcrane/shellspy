package shellspy

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync/atomic"
)

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
	fmt.Fprintln(conn, "Welcome to the remote shell!")
	transcriptLogName := fmt.Sprint(s.TranscriptCounter.Add(1))
	pathname := fmt.Sprintf("%s/transcript-%s.txt", s.TranscriptDirectory, transcriptLogName)
	session := NewSpySession(WithConnection(conn), WithTranscriptPath(pathname), WithServerLogger(&s.Logger))
	err := session.Start()
	if err != nil {
		s.Log(err)
	}
	fmt.Fprintln(conn, "Goodbye!")
}

// Log writes args to the [Server.Logger].
func (s *Server) Log(args ...any) {
	fmt.Fprintln(s.Logger, args...)
}

// Logf formats args into str and logs via [Server.Logger].
func (s *Server) Logf(str string, args ...any) {
	fmt.Fprintf(s.Logger, str, args...)
}

// ListenAndServe starts listening on the supplied port.
// It does not return until the server is shutdown.
func ListenAndServe(addr, serverPassword, logDir string) error {
	s := NewServer(addr, serverPassword, logDir)
	return s.ListenAndServe()
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
