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
	"strings"

	"chromiumos/tast/local/bundles/cros/printer/lpprint"
	"chromiumos/tast/local/bundles/cros/printer/proxylpprint"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/printing/document"
	"chromiumos/tast/testing"
)

// Params struct used by all ipp print tests for parameterized tests.
type Params struct {
	PPDFile      string   // Name of the ppd used to print the job.
	PrintFile    string   // The file to print.
	ExpectedFile string   // The file output should be compared to.
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

// run runs the given print function and compares the output to the golden file.
func run(ctx context.Context, s *testing.State, p *Params, printFun func(context.Context) ([]byte, error)) {
	expect, err := ioutil.ReadFile(s.DataPath(p.ExpectedFile))
	if err != nil {
		s.Fatal("Failed to read golden file: ", err)
	}
	request, err := printFun(ctx)
	if err != nil {
		s.Fatal("Print job failed: ", err)
	}
	if document.CleanContents(string(expect)) != document.CleanContents(string(request)) {
		outPath := filepath.Join(s.OutDir(), p.ExpectedFile)
		if err := ioutil.WriteFile(outPath, request, 0644); err != nil {
			s.Error("Failed to dump output: ", err)
		}
		s.Errorf("Printer output differs from expected: output saved to %q", p.ExpectedFile)
	}
}
