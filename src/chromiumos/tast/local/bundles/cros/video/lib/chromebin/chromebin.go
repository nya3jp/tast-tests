// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chromebin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// RunChromeTestBinary executes a chrome binary test, binaryToRun, with extraParams. This returns error if
// the chrome binary test fails.
func RunChromeTestBinary(ctx context.Context, binaryToRun string, extraParams []string) error {
	binaryTestPath := getBinaryTestPath(binaryToRun)

	// Binary test is executed as chronos.
	cmd := testexec.CommandContext(ctx, "su", "chronos", "-c", binaryTestPath+" "+strings.Join(extraParams, " "))
	cmd.Env = append(os.Environ(),
		"CHROME_DEVEL_SANDBOX=/opt/google/chrome/chrome-sandbox",
	)
	// Outputs test log in any case.
	defer cmd.DumpLog(ctx)

	testing.ContextLogf(ctx, "Executing %s %s", binaryToRun, extraParams)
	if err := cmd.Run(); err != nil {
		exitCode, isExitError := extractExitCode(err)
		if !isExitError {
			return errors.Wrapf(err, "error occurs in executing %s %s", binaryTestPath, extraParams)
		}
		if exitCode == 126 {
			perm, permErr := getPermission(binaryTestPath)
			if permErr != nil {
				errors.Wrapf(permErr, "failed to get permission: %v", err)
			}
			return errors.Wrapf(err, "cannot execute command %s (Permission: %s)", binaryTestPath, perm)
		} else {
			// TODO(crbug.com/889496): Parse gtest.xml here.
			return errors.Wrapf(err, "fail reason (%d)", exitCode)
		}
	}
	return nil
}

// getBinaryTestPath returns an absolute path of binaryToRun.
func getBinaryTestPath(binaryToRun string) string {
	const binaryTestDir = "/usr/local/libexec/chrome-binary-tests/"
	return filepath.Join(binaryTestDir, binaryToRun)
}

// getPermission returns an access permission of file.
func getPermission(file string) (string, error) {
	var info os.FileInfo
	var err error
	if info, err = os.Stat(file); err != nil {
		return "", err
	}
	mode := uint32(info.Mode())
	var perm [9]byte
	var i, j uint
	for i = 0; i < 3; i++ {
		const xwr = "xwr"
		for j = 0; j < 3; j++ {
			idx := 3*i + j
			if mode&(1<<idx) != 0 {
				perm[8-idx] = xwr[j]
			} else {
				perm[8-idx] = '-'
			}
		}
	}
	return string(perm[:]), nil
}

// extraExitCode returns an exit code composed in err.
func extractExitCode(err error) (int, bool) {
	if exiterr, ok := err.(*exec.ExitError); ok {
		if st, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			return st.ExitStatus(), true
		}
	}
	return -1, false
}
