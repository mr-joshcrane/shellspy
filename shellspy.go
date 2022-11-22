package shellspy

import (
	"fmt"
	"io"
	"os/exec"

	"bitbucket.org/creachadair/shell"
)

func CommandFromString(s string) (*exec.Cmd, error) {
	commands, ok := shell.Split(s)
	if !ok {
		return nil, fmt.Errorf("unbalanced quotes or backslashes in [%s]", s)
	}
	if len(commands) == 0 {
		return nil, fmt.Errorf("command length is 0")
	}
	path := commands[0]
	args := commands[1:]
	return exec.Command(path, args...), nil
}

func SpySession(r io.Reader, w io.Writer) {

}
