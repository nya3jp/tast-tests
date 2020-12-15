// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ippprint implements printing with IPP options.
package ippprint

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/local/bundles/cros/printer/lpprint"
	"chromiumos/tast/local/bundles/cros/printer/proxylpprint"
	"chromiumos/tast/testing"
)

// Option is for supplying filter options.
type Option string

// Params struct used by all ipp print tests for parameterized tests.
type Params struct {
	PpdFile      string   // Name of the ppd used to print the job.
	PrintFile    string   // PS file to print.
	ExpectedFile string   // PS file output should be compared to.
	Options      []Option // Options to be passed to the filter to change output.
	UseProxy     bool     // If enabled, print via the CUPS proxy.
}

// Collate enables collation.
func Collate() Option {
	return Option("multiple-document-handling=separate-documents-collated-copies")
}

// WithCopies properly formats a copies option.
func WithCopies(n int) Option {
	return Option(fmt.Sprintf("copies=%d", n))
}

// WithJobPassword properly formats a job-password option.
func WithJobPassword(pass string) Option {
	return Option(fmt.Sprintf("job-password=%s", pass))
}

// WithResolution properly formats a printer-resolution option.
func WithResolution(res string) Option {
	return Option(fmt.Sprintf("printer-resolution=%s", res))
}

// optionsToString turns an array of options into a space-delimited string.
func optionsToString(options []Option) string {
	var arr []string
	for _, o := range options {
		arr = append(arr, string(o))
	}
	return strings.Join(arr, " ")
}

// Run executes the main test logic with |p.Options| included in the lp command.
func Run(ctx context.Context, s *testing.State, p *Params) {
	run := lpprint.RunWithOptions
	if p.UseProxy {
		run = proxylpprint.RunWithOptions
	}
	run(ctx, s, p.PpdFile, p.PrintFile, p.ExpectedFile, optionsToString(p.Options))
}
