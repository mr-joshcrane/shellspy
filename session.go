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

type session struct {
	input          io.Reader
	terminal       io.Writer
	transcript     io.Writer
	combinedOutput io.Writer
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
