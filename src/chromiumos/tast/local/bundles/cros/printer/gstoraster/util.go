// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gstoraster

import (
	"context"
	"io"
	"io/ioutil"
	"path/filepath"

	"chromiumos/tast/diff"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// getFileContents loads the contents of the file f and returns it as a string.
func getFileContents(f string) (string, error) {
	bytes, err := ioutil.ReadFile(f)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read file %s", f)
	}
	return string(bytes), nil
}

// RunGstorasterTest executes a test on the gstoraster CUPS filer. It tests that
// gstoraster works correctly by running it with the file inputPdf as input, and
// verify that the produces output matches the contents of the golden file given
// at goldenPath.
func RunGstorasterTest(ctx context.Context, s *testing.State, inputPdf, goldenPath string) {
	cmd := testexec.CommandContext(ctx, "/usr/libexec/cups/filter/gstoraster", "1" /*jobID*/, "chronos" /*user*/, "Untitled" /*title*/, "1" /*copies*/, "" /*options*/)

	inputContents, err := getFileContents(inputPdf)
	if err != nil {
		s.Fatal("Failed to load file contents: ", err)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		s.Fatal("Failed to open stdin pipe for gstoraster command: ", err)
	}

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, inputContents)
	}()

	output, err := cmd.Output()
	if err != nil {
		s.Fatal("Failed to run gstoraster command: ", err)
	}

	diffPath := filepath.Join(s.OutDir(), "diff.txt")
	if err := compareFiles(ctx, string(output), goldenPath, diffPath); err != nil {
		s.Error("Printed filed differs from golden file: ", err)
	}
}

// compareFiles compare the string outputContents to the contents of the file
// golden and returns an error if there are any differences. If there are any
// differences between the compared file contents, then the results of the diff
// are written to diffPath.
func compareFiles(ctx context.Context, outputContents, golden, diffPath string) error {
	goldenContents, err := getFileContents(golden)
	if err != nil {
		return err
	}

	testing.ContextLog(ctx, "Comparing gstoraster output and ", golden)
	diff, err := diff.Diff(outputContents, goldenContents)
	if err != nil {
		return errors.Wrap(err, "unexpected diff output")
	}
	if diff != "" {
		testing.ContextLog(ctx, "Dumping diff to ", diffPath)
		if err := ioutil.WriteFile(diffPath, []byte(diff), 0644); err != nil {
			testing.ContextLog(ctx, "Failed to dump diff: ", err)
		}
		return errors.New("result file did not match the expected file")
	}
	return nil
}
