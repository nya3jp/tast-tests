// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"fmt"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SELinuxFileLabel,
		Desc:         "Checks that SELinux file labels are set correctly.",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"selinux"},
	})
}

func SELinuxFileLabel(s *testing.State) {
	getFileLabel := func(path string) (string, error) {
		b, err := testexec.CommandContext(s.Context(), "getfilecon", path).CombinedOutput()
		if err != nil {
			return "", err
		} else {
			bArray := strings.Split(strings.Trim(string(b), "\n"), "\t")
			if len(bArray) == 2 {
				return strings.Split(strings.Trim(string(b), "\n"), "\t")[1], nil
			}
			return "", fmt.Errorf("Unexpected getfilecon result %q", b)
		}
	}

	assertSELinuxFileContext := func(path string, expected string) {
		actual, err := getFileLabel(path)
		if err != nil {
			s.Error("Fail to get file context: ", err)
			return
		}
		if actual != expected {
			s.Errorf(
				"File context mismatch for file %s, expect %q, actual %q",
				path,
				expected,
				actual)
		}
	}

	assertSELinuxFileContext("/sbin/init", "u:object_r:chromeos_init_exec:s1")
}
