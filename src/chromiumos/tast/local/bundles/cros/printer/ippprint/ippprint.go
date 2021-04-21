// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ippprint implements printing with IPP options.
package ippprint

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"

	"chromiumos/tast/local/bundles/cros/printer/lpprint"
	"chromiumos/tast/local/bundles/cros/printer/proxylpprint"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/printing/document"
	"chromiumos/tast/testing"
)

// Params struct used by all ipp print tests for parameterized tests.
// Either ExpectedFile or ExpectedSize should be provided but not both.
type Params struct {
	PPDFile      string   // Name of the ppd used to print the job.
	PrintFile    string   // The file to print.
	ExpectedFile string   // The file output should be compared to.
	ExpectedSize int      // The size output should be compared to.
	Options      []string // Options to be passed to the filter to change output.
}

// Collate enables collation.
func Collate() string {
	return "multiple-document-handling=separate-documents-collated-copies"
}

// WithCopies properly formats a copies option.
func WithCopies(n int) string {
	return fmt.Sprintf("copies=%d", n)
}

// WithJobPassword properly formats a job-password option.
func WithJobPassword(pass string) string {
	return fmt.Sprintf("job-password=%s", pass)
}

// WithResolution properly formats a printer-resolution option.
func WithResolution(res string) string {
	return fmt.Sprintf("printer-resolution=%s", res)
}

// Run executes the main test logic with p.Options included in the lp command.
func Run(ctx context.Context, s *testing.State, p *Params) {
	run(ctx, s, p, func(ctx context.Context) ([]byte, error) {
		return lpprint.Run(ctx, s.DataPath(p.PPDFile), s.DataPath(p.PrintFile), strings.Join(p.Options, " "))
	})
}

// ProxyRun is similar to Run but uses proxylppprint instead of lpprint.
func ProxyRun(ctx context.Context, s *testing.State, p *Params) {
	run(ctx, s, p, func(ctx context.Context) ([]byte, error) {
		return proxylpprint.Run(ctx, s.PreValue().(*chrome.Chrome), s.DataPath(p.PPDFile), s.DataPath(p.PrintFile), strings.Join(p.Options, " "))
	})
}

// run runs the given print function and compares the output to the golden file
// (if p.ExpectedFile) or file size (if p.ExpectedSize).

func run(ctx context.Context, s *testing.State, p *Params, printFun func(context.Context) ([]byte, error)) {
	if p.ExpectedFile != "" {
		runWithFile(ctx, s, p, printFun)
		return
	}
	if p.ExpectedSize > 0 {
		runWithSize(ctx, s, p, printFun)
		return
	}
	s.Fatal("Invalid test parameters - both ExpectedFile and ExpectedSize omitted")
}

func runWithFile(ctx context.Context, s *testing.State, p *Params, printFun func(context.Context) ([]byte, error)) {
	expect, err := ioutil.ReadFile(s.DataPath(p.ExpectedFile))
	if err != nil {
		s.Fatal("Failed to read golden file: ", err)
	}
	request, err := printFun(ctx)
	if err != nil {
		s.Fatal("Print job failed: ", err)
	}
	if CleanPSContents(string(expect)) != CleanPSContents(string(request)) {
		outPath := filepath.Join(s.OutDir(), p.ExpectedFile)
		if err := ioutil.WriteFile(outPath, request, 0644); err != nil {
			s.Error("Failed to dump output: ", err)
		}
		s.Errorf("Printer output differs from expected: output saved to %q", p.ExpectedFile)
	}
}

func runWithSize(ctx context.Context, s *testing.State, p *Params, printFun func(context.Context) ([]byte, error)) {
	request, err := printFun(ctx)
	if err != nil {
		s.Fatal("Print job failed: ", err)
	}
	p.ExpectedFile = "output.bin"
	if len(request) != p.ExpectedSize {
		outPath := filepath.Join(s.OutDir(), p.ExpectedFile)
		if err := ioutil.WriteFile(outPath, request, 0644); err != nil {
			s.Error("Failed to dump output: ", err)
		}
		s.Errorf("Printer output (%d bytes) differs from expected (%d bytes): output saved to %q", p.ExpectedSize, len(request), p.ExpectedFile)
	}
}

// CleanPSContents filters any unwanted lines from content to ensure a stable
// diff.
func CleanPSContents(content string) string {
	r := regexp.MustCompile(
		// Matches the embedded ghostscript version in the PS file.
		// This gets outdated on every gs uprev, so we strip it out.
		`(?m)(^(%%Creator: GPL Ghostscript .*` +
			// Removes the postscript creation date.
			`|%%CreationDate: D:.*` +
			// Removes the ghostscript invocation command.
			`|%%Invocation: .*` +
			// Removes additional lines of the ghostscript invocation command.
			`|%%\+ .*` +
			// Removes time metadata for PCLm Jobs.
			`|% *job-start-time: .*` +
			// Removes PDF xref objects (they contain byte offsets).
			`|\d{10} \d{5} [fn] *` +
			// Removes the byte offset of a PDF xref object.
			`|startxref[\r\n]+\d+[\r\n]+%%EOF` +
			// For Brother jobs, jobtime and printlog item 2 contain
			// time-specific values.
			`|@PJL SET JOBTIME = .*` +
			`|@PJL PRINTLOG ITEM = 2,.*` +
			// For HP jobs, JobAcct4,JobAcc5 & DMINFO contain
			// time-specific values.
			`|@PJL SET JOBATTR="JobAcct[45]=.*` +
			`|@PJL DMINFO ASCIIHEX=".*` +
			// For Ricoh jobs, the SET DATE/TIME values are time-specific.
			`|@PJL SET DATE=".*` +
			`|@PJL SET TIME=".*)[\r\n])` +
			// For Ricoh jobs, the /ID tag is time-specific.
			`|\/ID \[<.*>\]` +
			// For Ricoh jobs, "usercode (\d+)" contains the date
			// and time of the print job.
			`|usrcode \(\d+\)` +
			// For Ricoh PS jobs, the time is contained here.
			`|/Time \(\d+\)` +
			// For Ricoh jobs, "(\d+) lppswd" contains the date
			// and time of the print job.
			`|\(\d+\) lppswd`)
	return r.ReplaceAllLiteralString(document.CleanPDFContents(content), "")
}
