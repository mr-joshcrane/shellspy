package shellspy_test

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/mr-joshcrane/shellspy"
)

func TestCommandFromString_ConvertsStringIntoExecutableCmd(t *testing.T) {
	t.Parallel()
	got := shellspy.CommandFromString("ls -l").Args
	want := []string{"ls", "-l"}
	if !cmp.Equal(want, got) {
		t.Fatalf(cmp.Diff(want, got))
	}
}

func TestCommandFromString_WithNoArgsConvertsToExecutableCmd(t *testing.T) {
	t.Parallel()
	got := shellspy.CommandFromString("echo").Args

	want := []string{"echo"}
	if !cmp.Equal(want, got) {
		t.Fatalf(cmp.Diff(want, got))
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
