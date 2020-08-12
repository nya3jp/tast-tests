// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ocr

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/local/printing/document"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: GenerateSearchablePDFFromImage,
		Desc: "Check that we can generate searchable PDF files from images",
		Contacts: []string{
			"emavroudi@google.com",
			"jschettler@google.com",
			"project-bolton@google.com ",
		},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{sourceImage, goldenPDF},
		SoftwareDeps: []string{"ocr"},
	})
}

const (
	sourceImage = "phototest.tif"
	goldenPDF   = "phototest_golden.pdf"
)

// GenerateSearchablePDFFromImage runs the ocr_tool on a test image and compares
// the output PDF file with a golden file. It also dumps the output to a file
// for debugging.
func GenerateSearchablePDFFromImage(ctx context.Context, s *testing.State) {
	tmpDir, err := ioutil.TempDir("", "tast.ocr.GenerateSearchablePDFFromImage.")
	if err != nil {
		s.Fatal("Failed to create temporary directory: ", err)
	}
	defer os.RemoveAll(tmpDir)

	outputPDFPath := filepath.Join(tmpDir, "phototest.pdf")
	cmd := testexec.CommandContext(ctx, "ocr_tool",
		"--input_image_filename="+s.DataPath(sourceImage), "--output_pdf_filename="+outputPDFPath,
		"--language=eng")
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("Failed to run %q: %v", shutil.EscapeSlice(cmd.Args), err)
	}

	// Log output to file for debugging.
	path := filepath.Join(s.OutDir(), "command_output.txt")
	if err := ioutil.WriteFile(path, out, 0644); err != nil {
		s.Fatal("Failed to write output to ", path)
	}

	diffPath := filepath.Join(s.OutDir(), "diff.txt")
	if err := document.CompareFiles(ctx, outputPDFPath, s.DataPath(goldenPDF), diffPath); err != nil {
		s.Error("Generated PDF file differs from golden file: ", err)
	}

}
