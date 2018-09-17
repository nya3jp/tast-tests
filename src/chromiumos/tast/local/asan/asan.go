// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package asan

import (
	"context"
	"os/exec"

	"chromiumos/tast/local/testexec"
)

const (
	asan_symbol = "__asan_init"
)

// Returns whether we're running on ASan.
func RunningOnAsan(ctx context.Context) (bool, error) {
	debugd, err := exec.LookPath("debugd")
	if err != nil {
		return false, err
	}

	// -q, --quiet         * Only output 'bad' things
	// -F, --format <arg>  * Use specified format for output
	// -g, --gmatch        * Use regex rather than string compare (with -s)
	// -s, --symbol <arg>  * Find a specified symbol
	cmd := testexec.CommandContext(
		ctx, "scanelf", "-qF'%s#F'", "-gs", asan_symbol, debugd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		cmd.DumpLog(ctx) // Ignore error on error.
		return false, err
	}
	return string(output) != "", nil
}
