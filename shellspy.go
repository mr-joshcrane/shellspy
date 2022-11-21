package shellspy

import (
	"os/exec"
	"strings"
)

func CommandFromString(s string) (*exec.Cmd) {
	commands := strings.Split(s, " ")
	path := commands[0]
	args := commands[1:]
	return exec.Command(path, args...)
}
