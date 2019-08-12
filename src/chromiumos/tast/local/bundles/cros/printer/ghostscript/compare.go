// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ghostscript

import (
	"context"
	"io/ioutil"
	"regexp"

	"chromiumos/tast/diff"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// pdfIDRegex matches the "ID" embedded in the PDF file which uniquely
// identifies the document. This line is removed so that file comparison will
// pass.
var pdfIDRegex = regexp.MustCompile("(?m)^\\/ID \\[<[A-F0-9]+><[A-F0-9]+>\\]$")

// creationDateRegex matches the "CreationDate" field embedded in the PDF file.
// This field is removed so that the file comparison will pass.
var creationDateRegex = regexp.MustCompile("(?m)^\\/CreationDate\\(D:[0-9]{14}-[0-9]{2}'[0-9]{2}'\\)$")

// modDateRegex matches the "ModDate" field embedded in the PDF file. This field
// is removed so that file comparison will pass.
var modDateRegex = regexp.MustCompile("(?m)^\\/ModDate\\(D:[0-9]{14}-[0-9]{2}'[0-9]{2}'\\)$")

func cleanPdfContents(contents string) string {
	cleared := contents
	for _, r := range []*regexp.Regexp{pdfIDRegex, creationDateRegex, modDateRegex} {
		cleared = r.ReplaceAllLiteralString(cleared, "")
	}
	return cleared
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

	outputContents = cleanPdfContents(outputContents)
	goldenContents := cleanPdfContents(string(goldenBytes))

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
