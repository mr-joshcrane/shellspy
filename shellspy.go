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

type Password *string

func NewPassword(pw string) Password {
	if pw == "" {
		return nil
	}
	return &pw
}

type Session struct {
	r          io.Reader
	output     io.Writer
	Transcript io.Writer
	Closed     bool
	Identity   string
}

func SpySession(r io.Reader, w io.Writer) Session {
	return Session{
		r:          r,
		output:     w,
		Transcript: io.Discard,
		Closed:     false,
		Identity:   "anonymous",
	}
}

func NewSpySession() Session {
	return SpySession(os.Stdin, os.Stdout)
}

func (s *Session) Auth(serverPassword Password, logger io.Writer) bool {
	if logger == nil {
		logger = io.Discard
	}
	if serverPassword == nil {
		fmt.Fprintln(logger, "No password required.")
		return true
	}
	fmt.Fprintln(s.output, "Enter Password: ")
	scan := bufio.NewScanner(s.r)
	for scan.Scan() {
		userPassword := scan.Text()
		if userPassword == *serverPassword {
			fmt.Fprintf(logger, "SUCCESSFUL LOGIN from %s\n", s.Identity)
			return true
		}
		break
	}
	fmt.Fprintln(s.output, "Incorrect Password: Closing connection")
	s.Closed = true
	fmt.Fprintf(logger, "FAILED LOGIN from %s\n", s.Identity)
	return false
}

func (s Session) Start() error {
	w := io.MultiWriter(s.output, s.Transcript)
	fmt.Fprint(s.output, "$ ")
	scan := bufio.NewScanner(s.r)

	for scan.Scan() {
		line := scan.Text()
		if line == "exit" {
			fmt.Fprintf(w, "exit\n")
			return io.EOF
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
	return scan.Err()
}

func ListenAndServe(addr string, serverPassword Password, logger io.Writer) error {
	fmt.Fprintf(logger, "Starting listener on %s\n", addr)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintln(logger, err)
		return err
	}
	fmt.Fprintf(logger, "Listener created.\n")
	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Fprintln(logger, err)
			return fmt.Errorf("Connection error: %q", err)
		}
		fmt.Fprintf(logger, "Accepting connection from %s\n", conn.RemoteAddr().String())
		go func(conn net.Conn) {
			defer conn.Close()
			session := SpySession(conn, conn)
			session.Identity = conn.RemoteAddr().String()
			authenticated := session.Auth(serverPassword, logger)
			if !authenticated {
				return
			}
			_, err = fmt.Fprintln(conn, "Welcome to the remote shell!")
			if err != nil {
				panic(err)
			}
			err = session.Start()
			if err != nil && err != io.EOF {
				panic(err)
			}
			fmt.Fprintln(conn, "Goodbye!")
		}(conn)
	}
}
