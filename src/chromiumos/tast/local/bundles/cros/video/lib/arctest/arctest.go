// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arctest handles ARC video test procedures.
package arctest

import (
	"context"
	"io"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// StartARCBinary starts running the command at ARC and returns the command object.
func StartARCBinary(ctx context.Context, a *arc.ARC, exec string, args []string, outFile io.Writer) (*testexec.Cmd, error) {
	cmd := a.Command(ctx, exec, args...)
	cmd.Stdout = outFile

	testing.ContextLogf(ctx, "Running %q", shutil.EscapeSlice(cmd.Args))
	return cmd, cmd.Start()
}

// RunARCBinary runs exec once and produces gtest xml output and log files.
// Always report by --gtest_output because we cannot rely on the return value of the adb command to
// determine whether the test passes (which is always 0). Parse from gtest output as alternative.
func RunARCBinary(ctx context.Context, a *arc.ARC, exec string, args []string, outDir string, outFile io.Writer) error {
	xmlPath := filepath.Join(arc.ARCTmpDirPath, filepath.Base(exec)+".xml")
	execArgs := append(args, "--gtest_output=xml:"+xmlPath)

	cmd, err := StartARCBinary(ctx, a, exec, execArgs, outFile)
	if err != nil {
		return errors.Wrapf(err, "failed to start running %v", exec)
	}
	if err := cmd.Wait(); err != nil {
		return errors.Wrapf(err, "failed waiting for %v to exit", exec)
	}

	if err := a.PullFile(ctx, xmlPath, outDir); err != nil {
		return errors.Wrapf(err, "failed to pull file %v from ARC", xmlPath)
	}

	report, err := gtest.ParseReport(filepath.Join(outDir, filepath.Base(xmlPath)))
	if err != nil {
		return errors.Wrap(err, "failed to parse gtest report")
	}

	failures := report.Failures()
	if len(failures) > 0 {
		return errors.Errorf("failed to run %v with %d test failure(s): %v",
			exec, len(failures), strings.Join(failures, ", "))
	}
	return nil
}
