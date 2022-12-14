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

func (s *Session) Auth(serverPassword Password) bool {
	if serverPassword == nil {
		return true
	}
	fmt.Fprintln(s.output, "Enter Password: ")
	scan := bufio.NewScanner(s.r)
	for scan.Scan() {
		userPassword := scan.Text()
		if userPassword == *serverPassword {
			return true
		}
		break
	}
	fmt.Fprintln(s.output, "Incorrect Password: Closing connection")
	s.Closed = true
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

func ListenAndServe(addr string, serverPassword Password) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("Connection error: %q", err)
		}
		go func(conn net.Conn) {
			defer conn.Close()
			session := SpySession(conn, conn)
			authenticated := session.Auth(serverPassword)
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
			return
		}(conn)

	}
}
