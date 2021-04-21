// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ippprint implements printing with IPP options.
package ippprint

import (
	"context"
	"fmt"
	"log"
	"strings"

	"chromiumos/tast/local/bundles/cros/printer/lpprint"
	"chromiumos/tast/local/bundles/cros/printer/proxylpprint"
	"chromiumos/tast/testing"
)

// Params struct used by all ipp print tests for parameterized tests.
type Params struct {
	PpdFile      string   // Name of the ppd used to print the job.
	PrintFile    string   // The file to print.
	ExpectedFile string   // The file output should be compared to.
	Options      []string // Options to be passed to the filter to change output.
}

// ColorType enumeration for print-color-mode values.
type ColorType int

const (
	// Auto print-color-mode=auto
	Auto ColorType = iota
	// Monochrome print-color-mode=monochrome aka grayscale
	Monochrome
	// Color print-color-mode=color generally RGB
	Color
)

// Collate enables collation.
func Collate() string {
	return "multiple-document-handling=separate-documents-collated-copies"
}

// WithColor formats a print-color-mode option.
func WithColor(color ColorType) string {
	value := ""
	switch color {
	case Auto:
		value = "auto"
	case Monochrome:
		value = "monochrome"
	case Color:
		value = "color"
	default:
		log.Fatal("Unrecognized Color enum")
		panic("programmer error")
	}

	return fmt.Sprintf("print-color-mode=%s", value)
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
	lpprint.RunWithOptions(ctx, s, p.PpdFile, p.PrintFile, p.ExpectedFile, strings.Join(p.Options, " "))
}

// ProxyRun is similar to Run but uses proxylppprint instead of lpprint.
func ProxyRun(ctx context.Context, s *testing.State, p *Params) {
	proxylpprint.RunWithOptions(ctx, s, p.PpdFile, p.PrintFile, p.ExpectedFile, strings.Join(p.Options, " "))
}
