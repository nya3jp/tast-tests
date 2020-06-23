// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package device contains device-related test logic shared by audio tests.
package cras_test_utils

import (
	"context"
	"strconv"

	"chromiumos/tast/local/testexec"
)

func CRASPlaybackCommand(ctx context.Context, duration int, blocksize int) (cmd *testexec.Cmd) {
	// Playback function by CRAS.
	command := testexec.CommandContext(
		ctx, "cras_test_client",
		"--playback_file", "/dev/zero",
		"--duration", strconv.Itoa(duration),
		"--num_channels", "2",
		"--block_size", strconv.Itoa(blocksize),
		"--rate", "48000")

	return command
}

func CRASCaptureCommand(ctx context.Context, duration int, blocksize int) (cmd *testexec.Cmd) {
	// Playback function by CRAS.
	command := testexec.CommandContext(
		ctx, "cras_test_client",
		"--capture_file", "/dev/null",
		"--duration", strconv.Itoa(duration),
		"--num_channels", "2",
		"--block_size", strconv.Itoa(blocksize),
		"--rate", "48000")

	return command
}
