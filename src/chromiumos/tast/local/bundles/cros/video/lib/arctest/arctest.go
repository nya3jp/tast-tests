// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arctest handles ARC video test procedures.
package arctest

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/camera/hal3"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// RunARCBinary runs exec once and produces gtest xml output and log files.
// Always report by --gtest_output because we cannot rely on the return value of the adb command to
// determine whether the test passes (which is always 0). Parse from gtest output as alternative.
func RunARCBinary(ctx context.Context, a *arc.ARC, exec string, args []string, outDir string) error {
	xmlPath := filepath.Join(arc.ARCTmpDirPath, filepath.Base(exec)+".xml")
	execArgs := append(args, "--gtest_output=xml:"+xmlPath)

	out, err := os.Create(filepath.Join(outDir, filepath.Base(exec)+".log"))
	if err != nil {
		return errors.Wrap(err, "failed to create output log file")
	}
	defer out.Close()

	cmd := a.Command(ctx, exec, execArgs...)
	cmd.Stdout = out

	testing.ContextLog(ctx, "Running ", shutil.EscapeSlice(cmd.Args))
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to run %v", exec)
	}

	if err := a.PullFile(ctx, xmlPath, outDir); err != nil {
		return errors.Wrapf(err, "failed to pull file %v from ARC", xmlPath)
	}
	xmlOutPath := filepath.Join(outDir, filepath.Base(xmlPath))
	xml, err := ioutil.ReadFile(xmlOutPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read file %v", xmlOutPath)
	}

	// extractFailedTests in tast/local/chrome/bintest cannot be used here because gtest has different
	// versions on Chrome and Chrome OS.
	// TODO(johnylin): use common gtest parse function instead once the gtest package is merged.
	//                 crbug.com/946390
	failures, err := hal3.GetFailedTestNames(strings.NewReader(string(xml)))
	if err != nil {
		return errors.Wrapf(err, "failed to get failed tests from %v", xmlOutPath)
	}
	if len(failures) > 0 {
		return errors.Errorf("failed to run %v with %d test failure(s): %v",
			exec, len(failures), strings.Join(failures, ", "))
	}
	return nil
}
