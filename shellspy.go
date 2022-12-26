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

func ListenAndServe(addr string) error {
	listener, err := retryListener(5, addr)
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
			passwordMsg := []byte("Enter Password: \n")
			conn.Write(passwordMsg)
			scan := bufio.NewScanner(conn)
			if scan.Scan() {
				password := scan.Text()
				if password != "password" {
					return
				}
			}
			welcomeMsg := []byte("Welcome to the remote shell!\n")
			conn.Write(welcomeMsg)
			session := SpySession(conn, conn)
			session.Start()
			goodbyeMsg := []byte("Goodbye!\n")
			conn.Write(goodbyeMsg)
		}(conn)

	}
}
