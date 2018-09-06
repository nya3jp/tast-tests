package security

import (
	"fmt"
	"os/exec"
	"strings"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SELinux,
		Desc: "SELinux test",
	})
}

func getFileLabel(path string, s *testing.State) (string, error) {
	b, err := exec.Command("getfilecon", path).CombinedOutput()
	if err != nil {
		return "", err
	} else {
		return strings.Split(strings.Trim(string(b), "\n"), "\t")[1], nil
	}
}

func assertSELinuxFileContext(path string, expected_context string, s *testing.State) {
	actual_context, err := getFileLabel(path, s)
	if err != nil {
		s.Error("fail to get file context", err)
	} else {
		if actual_context != expected_context {
			s.Error(
				fmt.Sprintf(
					"File context mismatch for file %s, expect %s, actual %s",
					path,
					expected_context,
					actual_context))

		}
	}
}

func SELinuxFileLabel(s *testing.State) {
	assertSELinuxFileContext("/sbin/init", "u:object_r:chromeos_init_exec:s0", s)
}

func SELinux(s *testing.State) {
	SELinuxFileLabel(s)
}
