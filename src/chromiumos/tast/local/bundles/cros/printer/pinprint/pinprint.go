// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pinprint implements PinPrint* tests.
package pinprint

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/diff"
	"chromiumos/tast/local/bundles/cros/printer/fake"
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/local/printing/printer"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Option is for supplying filter options
type Option string

// Params struct used by all pin print tests for parameterized tests.
type Params struct {
	PpdFile    string
	PrintFile  string
	GoldenFile string
	DiffFile   string
	Options    []Option
}

// WithJobPassword properly formats a job-password option
func WithJobPassword(pass string) Option {
	return Option(fmt.Sprintf("job-password=%s", pass))
}

// optionsToString turns an array of options into a space-delimited string
func optionsToString(options []Option) string {

	var arr []string
	for _, o := range options {
		arr = append(arr, string(o))
	}
	return strings.Join(arr, " ")
}

// cleanPSContents filters any unwanted lines from |content| to ensure a stable
// diff.
func cleanPSContents(content string) string {
	r := regexp.MustCompile(
		// Matches the embedded poppler version in the PS file. This gets
		// outdated on every poppler uprev, so we strip it out.
		"(?m)(^(.*poppler.*version:.*" +
			// For HP jobs, JobAcct4,JobAcc5 & DMINFO contain
			// time-specific values.
			"|@PJL SET JOBATTR=\"JobAcct[45]=.*" +
			"|@PJL DMINFO ASCIIHEX=\".*" +
			// For Ricoh jobs, the SET DATE/TIME values are time-specific.
			"|@PJL SET DATE=\".*" +
			"|@PJL SET TIME=\".*)[\r\n])" +
			// For Ricoh jobs, the /ID tag is time-specific.
			"|(\\/ID \\[<.*>\\])" +
			// For Ricoh jobs, "usercode (\d+)" contains the date
			// and time of the print job.
			"|(usrcode \\(\\d+\\))" +
			// For Ricoh PS jobs, the time is contained here.
			"|(/Time \\(\\d+\\))" +
			// For Ricoh jobs, "(\d+) lppswd" contains the date
			// and time of the print job.
			"|(\\(\\d+\\)) lppswd")
	return r.ReplaceAllLiteralString(content, "")
}

// Run executes the main test logic with |p.Options| included in the lp command.
func Run(ctx context.Context, s *testing.State, p *Params) {
	const printerID = "FakePrinterID"

	ppd, err := ioutil.ReadFile(s.DataPath(p.PpdFile))
	if err != nil {
		s.Fatal("Failed to read PPD file: ", err)
	}
	expect, err := ioutil.ReadFile(s.DataPath(p.GoldenFile))
	if err != nil {
		s.Fatal("Failed to read golden file: ", err)
	}

	if err := printer.ResetCups(ctx); err != nil {
		s.Fatal("Failed to reset cupsd: ", err)
	}

	fake, err := fake.NewPrinter(ctx)
	if err != nil {
		s.Fatal("Failed to start fake printer: ", err)
	}
	defer fake.Close()

	d, err := debugd.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to debugd: ", err)
	}

	testing.ContextLog(ctx, "Registering a printer")
	if result, err := d.CupsAddManuallyConfiguredPrinter(
		ctx, printerID, "socket://127.0.0.1/", ppd); err != nil {
		s.Fatal("Failed to call CupsAddManuallyConfiguredPrinter: ", err)
	} else if result != debugd.CUPSSuccess {
		s.Fatal("Could not set up a printer: ", result)
	}

	testing.ContextLog(ctx, "Issuing print request")
	var cmd *testexec.Cmd
	options := optionsToString(p.Options)
	if len(options) != 0 {
		cmd = testexec.CommandContext(ctx, "lp", "-d", printerID, "-o", options, s.DataPath(p.PrintFile))
	} else {
		cmd = testexec.CommandContext(ctx, "lp", "-d", printerID, s.DataPath(p.PrintFile))
	}
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to run lp: ", err)
	}

	testing.ContextLog(ctx, "Receiving print request")
	recvCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	request, err := fake.ReadRequest(recvCtx)
	if err != nil {
		s.Fatal("Fake printer didn't receive a request: ", err)
	}

	diff, err := diff.Diff(cleanPSContents(string(request)), cleanPSContents(string(expect)))
	if err != nil {
		s.Fatal("Unexpected diff output: ", err)
	}
	if diff != "" {
		path := filepath.Join(s.OutDir(), p.DiffFile)
		if err := ioutil.WriteFile(path, []byte(diff), 0644); err != nil {
			s.Error("Failed to dump diff: ", err)
		}

		// Write out the complete output.
		psPath := filepath.Join(s.OutDir(), strings.TrimSuffix(p.DiffFile, filepath.Ext(p.DiffFile))+".ps")
		if err := ioutil.WriteFile(psPath, []byte(cleanPSContents(string(request))), 0644); err != nil {
			s.Error("Failed to dump ps: ", err)
		}

		s.Errorf("Output diff from the golden file, diff at %s, output.ps at %s", p.DiffFile, psPath)
	}
}
