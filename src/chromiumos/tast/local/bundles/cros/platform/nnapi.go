// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: NNAPI,
		Desc: "Verifies that libneuralnetworks.so can be loaded by ml_cmdline",
		Contacts: []string{
			"jmpollock@google.com",
			"slangley@google.com",
			"chromeos-platform-ml@google.com",
		},
		Attr: []string{
			"group:mainline",
			"informational",
		},
		SoftwareDeps: []string{"nnapi"},
	})
}

func NNAPI(ctx context.Context, s *testing.State) {
	for i, tc := range []struct {
		args          []string // arguments to ml_cmdline
		expectedLines []string // set of lines to expect in output
	}{
		{nil, []string{"Adding 1 and 4 with CPU", "Status: OK", "Sum: 5"}},
		{[]string{"--nnapi"}, []string{"Adding 1 and 4 with NNAPI", "Status: OK", "Sum: 5"}},
	} {
		cmd := testexec.CommandContext(ctx, "ml_cmdline", tc.args...)
		var stderrBytes bytes.Buffer
		var stdoutBytes bytes.Buffer
		cmd.Stderr = &stderrBytes
		cmd.Stdout = &stdoutBytes

		if err := cmd.Run(); err != nil {
			s.Errorf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
		}

		stdout := stdoutBytes.String()
		stderr := stderrBytes.String()

		logFilename := fmt.Sprintf("nnapi_test-%d.log", i)
		logOutput(s, logFilename, shutil.EscapeSlice(cmd.Args), stdout, stderr)

		if strings.Contains(stdout, "error") || strings.Contains(stderr, "error") {
			s.Errorf("%s contained output with an error. See %s", shutil.EscapeSlice(cmd.Args), logFilename)
		} else if outSlice := strings.Split(stdout, "\n"); !containsAll(outSlice, tc.expectedLines) {
			s.Errorf("%s did not produce all of %q. See %s", shutil.EscapeSlice(cmd.Args), tc.expectedLines, logFilename)
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

// logOutput will write out the parameters to logFilename
func logOutput(s *testing.State, logFilename, cmd, stdout, stderr string) {
	logf, err := os.Create(filepath.Join(s.OutDir(), logFilename))
	if err != nil {
		s.Fatal("Failed to create logfile: ", err)
	}
	defer logf.Close()

	fmt.Fprintln(logf, cmd)
	fmt.Fprintln(logf, "stdout:")
	fmt.Fprintln(logf, stdout)
	fmt.Fprintln(logf, "stderr:")
	fmt.Fprintln(logf, stderr)
}
