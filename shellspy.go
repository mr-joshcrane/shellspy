package shellspy

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"time"

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

type Session struct {
	r          io.Reader
	output     io.Writer
	Transcript io.Writer
}

func SpySession(r io.Reader, w io.Writer) Session {
	return Session{
		r:          r,
		output:     w,
		Transcript: io.Discard,
	}
}

func NewSpySession() Session {
	return SpySession(os.Stdin, os.Stdout)
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

func RetryDial(retries int, addr string) (net.Conn, error) {
	for i := 0; i < retries; i++ {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			return conn, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, fmt.Errorf("retried %d times, to dial but was unsuccessful", retries)
}

func retryListener(retries int, addr string) (net.Listener, error) {
	for i := 0; i < retries; i++ {
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			return listener, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, fmt.Errorf("retried %d times to listen but was unsuccessful", retries)
}

func ListenAndServe(addr string, password string) error {
	listener, err := retryListener(5, addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	for {
		log, err := os.Create("log.txt")
		if err != nil {
			return fmt.Errorf("Error saving log: %q", err)
		}
		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("Connection error: %q", err)
		}
		go func(conn net.Conn) {
			fmt.Fprintln(conn, "Enter Password: ")
			scan := bufio.NewScanner(conn)
			if scan.Scan() {
				password := scan.Text()
				if password != password {
					conn.Close()
					return
				}
			}
			fmt.Fprintln(conn, "Welcome to the remote shell!")
			session := SpySession(conn, conn)
			session.Transcript = log
			session.Start()
			fmt.Fprintln(conn, "Goodbye!")
			conn.Close()
		}(conn)

	}
}
