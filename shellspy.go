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

func SpySession(r io.Reader, output io.Writer, transcript io.Writer) error {
	w := io.MultiWriter(output, transcript)
	fmt.Fprint(output, "$ ")
	scan := bufio.NewScanner(r)
	
	for scan.Scan() {
		line := scan.Text()
		fmt.Fprintf(transcript, "$ %s\n", line)
		cmd, err := CommandFromString(line)
		if err != nil {
			fmt.Fprintln(w, err)
			fmt.Fprint(transcript , err)
			fmt.Fprint(w, "$ ")
			fmt.Fprint(transcript, "$ ")
			continue
		}
		cmd.Stdout = w
		cmd.Stderr = w
		err = cmd.Run()
		if err != nil {
			fmt.Fprintln(w, err)
			fmt.Fprintln(transcript, err)
		}
		fmt.Fprint(output, "$ ")
	}
	return scan.Err()
}
