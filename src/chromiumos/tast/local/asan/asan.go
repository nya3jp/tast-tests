// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package asan containing utilities related to Address Sanitizer.
package asan

import (
	"context"
	"os/exec"

	"chromiumos/tast/local/testexec"
)

const (
	// If program is built under ASan enabled, this symbol should be
	// defined.
	asanSymbol = "__asan_init"
)

// Enabled returns whether ASan is enabled for the image.
func Enabled(ctx context.Context) (bool, error) {
	debugd, err := exec.LookPath("debugd")
	if err != nil {
		return false, err
	}

	// -q, --quiet         * Only output 'bad' things
	// -F, --format <arg>  * Use specified format for output
	// -g, --gmatch        * Use regex rather than string compare (with -s)
	// -s, --symbol <arg>  * Find a specified symbol
	output, err := testexec.CommandContext(
		ctx, "scanelf", "-qF'%s#F'", "-gs", asanSymbol, debugd).CombinedOutput(testexec.DumpLogOnError)
	if err != nil {
		return false, err
	}
	return string(output) != "", nil
}
