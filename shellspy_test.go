package shellspy_test

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/mr-joshcrane/shellspy"
)

func TestCommandFromString_ConvertsStringIntoExecutableCmd(t *testing.T) {
	t.Parallel()
	got := shellspy.CommandFromString("ls -l")

	want := exec.Command("ls", "-l")
	if !cmp.Equal(got, want, cmp.AllowUnexported(exec.Cmd{})) {
		t.Fatalf(cmp.Diff(got, want, cmp.AllowUnexported(exec.Cmd{})))
	}
}

func TestCommandFromString_WithNoArgsConvertsToExecutableCmd(t *testing.T) {
	t.Parallel()
	got := shellspy.CommandFromString("echo")

	want := exec.Command("echo")
	if !cmp.Equal(got, want, cmp.AllowUnexported(exec.Cmd{})) {
		t.Fatalf(cmp.Diff(got, want, cmp.AllowUnexported(exec.Cmd{})))
	}
}

func TestSpySession_StartsALoopThatTerminatesGivenExitFollowedByNewLine(t *testing.T) {
	t.Parallel()
	input := bytes.NewBufferString("echo 'one'\necho 'exit'\nexit\n")
	buf := bytes.NewBuffer([]byte{})
	shellspy.SpySession(input, buf)
	got := buf.String()
	want := "> echo 'one'\none\n\n> echo 'exit\n\nexit\n>exit\n"
	if want != got {
		t.Fatalf(cmp.Diff(want, got))
	}

}