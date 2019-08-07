// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"io/ioutil"
	"path/filepath"

	"chromiumos/tast/diff"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Gstoraster,
		Desc:         "Tests that the gstoraster CUPS filter produces expected output",
		Contacts:     []string{"valleau@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", "cups"},
		Data:         []string{"gstoraster_input.pdf", "gstoraster_golden.pwg"},
		Pre:          chrome.LoggedIn(),
	})
}

// compareFiles compare the string outputContents to the contents of the file
// golden and returns an error if there are any differences. If there are any
// differences between the compared file contents, then the results of the diff
// are written to diffPath.
func compareFiles(ctx context.Context, outputContents, golden, diffPath string) error {
	goldenBytes, err := ioutil.ReadFile(golden)
	if err != nil {
		return errors.Wrapf(err, "failed to read file %s", golden)
	}

	testing.ContextLog(ctx, "Comparing gstoraster output and ", golden)
	diff, err := diff.Diff(outputContents, string(goldenBytes))
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

func Gstoraster(ctx context.Context, s *testing.State) {
	inputContents, err := ioutil.ReadFile(s.DataPath("gstoraster_input.pdf"))
	if err != nil {
		s.Fatal("Failed to load file contents: ", err)
	}

	cmd := testexec.CommandContext(ctx, "/usr/libexec/cups/filter/gstoraster", "1" /*jobID*/, "chronos" /*user*/, "Untitled" /*title*/, "1" /*copies*/, "" /*options*/)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		s.Fatal("Failed to open stdin pipe for gstoraster command: ", err)
	}

	go func() {
		defer stdin.Close()
		if _, err := stdin.Write(inputContents); err != nil {
			s.Error("Failed to write to stdin pipe for gstoraster command: ", err)
		}
	}()

	output, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to run gstoraster command: ", err)
	}

	diffPath := filepath.Join(s.OutDir(), "diff.txt")
	if err := compareFiles(ctx, string(output), s.DataPath("gstoraster_golden.pwg"), diffPath); err != nil {
		s.Error("Printed filed differs from golden file: ", err)
	}
}
