// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"bytes"
	"context"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Nnapi,
		Desc: "Verifies that libneuralnetworks.so can be loaded by ml_cmdline",
		Contacts: []string{
			"jmpollock@google.com",
			"slangley@google.com",
		},
		Attr: []string{
			"group:mainline",
			"informational",
		},
	})
}

func Nnapi(ctx context.Context, s *testing.State) {
	for _, tc := range []struct {
		arg           string   // argument to ml_cmdline
		expectedLines []string // set of lines to expect in output
	}{
		{"", []string{"Adding 1 and 4 with CPU", "Status: OK", "Sum: 5"}},
		{"--nnapi", []string{"Adding 1 and 4 with NNAPI", "Status: OK", "Sum: 5"}},
	} {
		cmd := testexec.CommandContext(ctx, "ml_cmdline", tc.arg)
		var stderrBytes bytes.Buffer
		var stdoutBytes bytes.Buffer
		cmd.Stderr = &stderrBytes
		cmd.Stdout = &stdoutBytes

		if err := cmd.Run(); err != nil {
			s.Errorf("%q failed: %v", shutil.EscapeSlice(cmd.Args), err)
		}

		var stdout = stdoutBytes.String()
		var stderr = stderrBytes.String()

		if strings.Contains(stdout, "error") || strings.Contains(stderr, "error") {
			s.Errorf("%q printed stdout: %q and stderr: %q; looks like an error", shutil.EscapeSlice(cmd.Args), stdout, stderr)
		} else if outSlice := strings.Split(stdout, "\n"); !containsAll(outSlice, tc.expectedLines) {
			s.Errorf("%q printed %q; want all of %q, and no errors", shutil.EscapeSlice(cmd.Args), outSlice, tc.expectedLines)
		}
	}
}

// containsAll checks that sliceToQuery is a superset of sliceToMatch.
func containsAll(sliceToQuery, sliceToMatch []string) bool {
	for _, item := range sliceToMatch {
		if !contains(sliceToQuery, item) {
			return false
		}
	}
	return true
}

// contains checks that sliceToQuery contains an instance of toFind.
func contains(sliceToQuery []string, toFind string) bool {
	for _, item := range sliceToQuery {
		if item == toFind {
			return true
		}
	}
	return false
}
