package shellspy_test

import (
	"os/exec"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/mr-joshcrane/shellspy"
)

func TestCommandFromString_ConvertsStringIntoExecutableCmd(t *testing.T) {
	t.Parallel()
	got, err := shellspy.CommandFromString("ls -l")
	if err != nil {
		t.Fatal(err)
	}
	want := exec.Command("ls", "-l")
	if !cmp.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

}
