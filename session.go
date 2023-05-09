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
	transcriptPath string
	serverLogger   io.Writer
}

// Convenience wrapped around Session with default arguments.
func NewSpySession(opts ...SessionOption) *session {
	s := &session{
		input:        os.Stdin,
		terminal:     os.Stdout,
		serverLogger: os.Stdout,
	}
	for _, opt := range opts {
		opt(s)
	}
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

func WithTranscriptPath(path string) SessionOption {
	return func(s *session) *session {
		s.transcriptPath = path
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

func WithServerLogger(serverLogger io.Writer) SessionOption {
	return func(s *session) *session {
		s.serverLogger = serverLogger
		return s
	}
}

func (s session) printPromptToCombinedOutput() {
	fmt.Fprint(s.combinedOutput, "$ ")
}

func (s session) printMessageToUser(msg string) {
	fmt.Fprintln(s.terminal, msg)
}

func (s session) log(args ...any) {
	fmt.Fprintln(s.serverLogger, args...)
}

// Start reads from the [session] input
// and write to the [session] output. It will also
// write to the [session] transcript.
func (s session) Start() {
	if s.transcript == nil {
		s.transcript = io.Discard
		if s.transcriptPath == "" {
			s.printMessageToUser("No transcript requested")
		} else {
			transcript, err := os.Create(s.transcriptPath)
			if err != nil {
				s.printMessageToUser("WARNING No transcript will be available for this session!")
				s.log(err)
			} else {
				s.transcript = transcript
				s.log("Transcript for new session available at", s.transcriptPath)
			}
		}
	}
	s.combinedOutput = io.MultiWriter(s.terminal, s.transcript)
	s.printPromptToCombinedOutput()
	scan := bufio.NewScanner(s.input)
	for scan.Scan() {
		line := scan.Text()
		err := s.processLine(line)
		if err == io.EOF {
			break
		}
	}
	if scan.Err() != nil {
		s.log(scan.Err())
	}
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
	session := NewSpySession(WithTranscriptPath("transcript.txt"))
	session.Start()
	return 0
}
