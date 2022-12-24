package shellspy

import (
	"bufio"
	"fmt"
	"io"
	"log"
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

func ListenAndServe(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	for {
		go func(listener net.Listener) {
			conn, err := listener.Accept()
			if err != nil {
				log.Fatalf("Error with client connection: %q", err)
			}
			welcomeMsg := []byte("Welcome to the remote shell!\n")
			conn.Write(welcomeMsg)
			SpySession(conn, conn)
		}(listener)

	}
}
