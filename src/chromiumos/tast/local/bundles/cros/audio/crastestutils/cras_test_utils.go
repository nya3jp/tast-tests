// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crastestutils contains device-related test logic shared by audio tests.
package crastestutils

import (
	"context"
	"strconv"

	"chromiumos/tast/local/testexec"
)

// CRASPlaybackCommand creates a cras_test_client playback command.
func CRASPlaybackCommand(ctx context.Context, duration, blocksize int64) (cmd *testexec.Cmd) {
	// Playback function by CRAS.
	command := testexec.CommandContext(
		ctx, "cras_test_client",
		"--playback_file", "/dev/zero",
		"--duration", strconv.FormatInt(duration, 10),
		"--num_channels", "2",
		"--block_size", strconv.FormatInt(blocksize, 10),
		"--rate", "48000")

	return command
}

// CRASCaptureCommand creates a cras_test_client capture command.
func CRASCaptureCommand(ctx context.Context, duration, blocksize int64) (cmd *testexec.Cmd) {
	// Playback function by CRAS.
	command := testexec.CommandContext(
		ctx, "cras_test_client",
		"--capture_file", "/dev/null",
		"--duration", strconv.FormatInt(duration, 10),
		"--num_channels", "2",
		"--block_size", strconv.FormatInt(blocksize, 10),
		"--rate", "48000")

	return command
}
