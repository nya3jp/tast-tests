// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

func RunChromeTestBinary(ctx context.Context, outDir, binaryToRun, extraParams string) error {
	binaryTestPath := getBinaryTestPath(binaryToRun)
	if _, err := os.Stat(binaryTestPath); err != nil {
		return errors.Wrapf(err, "%s doesn't exist", binaryTestPath)
	}
	gtestXml := filepath.Join(outDir, "gtest.xml")
	cmd := testexec.CommandContext(ctx, binaryTestPath, extraParams)
	cmd.Env = append(os.Environ(),
		"CHROME_DEVEL_SANDBOX=/opt/google/chrome/chrome-sandbox",
		"GTEST_OUTPUT=xml:"+gtestXml,
	)
	// Is it needed to execute command as chronos here? If yes, How?
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		exitCode := extractExitCode(err)
		if exitCode == 0 {
			return errors.Wrapf(err, "error occurs in executing %s %s", binaryTestPath, extraParams)
		}
		if exitCode == 126 {
			perm, err := getPermission(binaryTestPath)
			return errors.Wrapf(err, "cannot execute command %s (Permission: %s)", binaryTestPath, perm)
		} else {
			failRes, err := parseFailReason(gtestXml)
			return errors.Wrapf(err, "fail reason (%d): %s", exitCode, failRes)
		}
	}
	return nil
}

func getBinaryTestPath(binary_to_run string) string {
	const binaryTestDir = "/usr/local/libexec/chrome-binary-tests/"
	return filepath.Join(binaryTestDir, binary_to_run)
}

func getPermission(file string) (string, error) {
	// TODO
	return "", nil
}

func extractExitCode(err error) int {
	if exiterr, ok := err.(*exec.ExitError); ok {
		if st, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			return st.ExitStatus()
		}
	}
	return 0
}

func parseFailReason(gtestXml string) (string, error) {
	// TODO: Need to parse XML here.
	return "", nil
}

// https://cs.corp.google.com/chromeos_public/src/third_party/autotest/files/client/cros/chrome_binary_test.py?q=chrome_binary+package:%5Echromeos_public$&dr=CSs&l=64
// https://cs.corp.google.com/chromeos_public/src/third_party/autotest/files/client/common_lib/utils.py
