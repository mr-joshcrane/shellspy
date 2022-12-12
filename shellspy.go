package shellspy

import (
	"bufio"
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
		return nil, fmt.Errorf("")
	}
	path := commands[0]
	args := commands[1:]
	return exec.Command(path, args...), nil
}

func SpySession(r io.Reader, w io.Writer, transcript io.Writer) error {
	fmt.Fprint(w, "$ ")

	scan := bufio.NewScanner(r)
	
	for scan.Scan() {
		cmd, err := CommandFromString(scan.Text())
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
		fmt.Fprint(w, "$ ")
	}
	return scan.Err()
}
