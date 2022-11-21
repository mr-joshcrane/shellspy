package shellspy

import "os/exec"

func CommandFromString(s string) (*exec.Cmd, error) {
	return exec.Command("ls", "-l"), nil
}
