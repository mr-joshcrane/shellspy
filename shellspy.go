package shellspy

import (
	"io"
	"os/exec"
	"strings"
)

func CommandFromString(s string) *exec.Cmd {
	commands := strings.Split(s, " ")
	path := commands[0]
	args := commands[1:]
	return exec.Command(path, args...)
}

func SpySession(r io.Reader, w io.Writer) {

}
