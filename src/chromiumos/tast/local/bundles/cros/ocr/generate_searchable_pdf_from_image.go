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
	"chromiumos/tast/local/upstart"
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

func GenerateSearchablePDFFromImage(ctx context.Context, s *testing.State) {
	// Runs the CLI command ocr_tool with arguments
	runOCR := func(ocrArgs ...string) string {
		cmd := testexec.CommandContext(ctx, "ocr_tool", ocrArgs...)
		s.Logf("Running %q", shutil.EscapeSlice(cmd.Args))
		out, err := cmd.Output()
		if err != nil {
			cmd.DumpLog(ctx)
			s.Fatalf("Failed to run %q: %v", shutil.EscapeSlice(cmd.Args), err)
		}
		return string(out)
	}

	tmpDir, err := ioutil.TempDir("", "tast.ocr.GenerateSearchablePDFFromImage.")
	if err != nil {
		s.Fatal("Failed to create temporary directory: ", err)
	}
	defer os.RemoveAll(tmpDir)

	outputPDFPath := filepath.Join(tmpDir, "phototest_eng.pdf")
	ocrArgs := []string{"--input_image_filename=" + s.DataPath(sourceImage), "--output_pdf_filename=" + outputPDFPath, "--language=eng"}
	runOCR(ocrArgs...)

	s.Log("Comparing generated PDF file to golden PDF file")
	diffPath := filepath.Join(s.OutDir(), "diff.txt")
	if err := document.CompareFiles(ctx, outputPDFPath, s.DataPath(goldenPDF), diffPath); err != nil {
		s.Error("Generated PDF file differs from golden file: ", err)
	}

	if err := upstart.CheckJob(ctx, "ocr_service"); err != nil {
		s.Fatal("Daemon job not running: ", err)
	}

}
