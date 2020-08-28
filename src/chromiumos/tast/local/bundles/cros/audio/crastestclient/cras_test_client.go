// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crastestclient contains cras_test_client utilities shared by audio tests.
package crastestclient

import (
	"context"
	"strconv"

	"chromiumos/tast/local/testexec"
)

type cmdMode int

const (
	captureMode cmdMode = iota
	playbackMode
)

// getBlockSize calculates default block size from rate. This should be aligned as defined in cras_test_client.
func getBlockSize(rate int) int {
	const playbackBufferedTimeUs = 5000
	return rate * playbackBufferedTimeUs / 1000000
}

// getCommand creates a cras_test_client command.
func getCommand(ctx context.Context, mode cmdMode, file string, duration, channels, blocksize, rate int) *testexec.Cmd {
	var runStr string
	if mode == captureMode {
		runStr = "--capture_file"
	} else { // playbackMode
		runStr = "--playback_file"
	}

	return testexec.CommandContext(
		ctx, "cras_test_client",
		runStr, file,
		"--duration", strconv.Itoa(duration),
		"--num_channels", strconv.Itoa(channels),
		"--block_size", strconv.Itoa(blocksize),
		"--rate", strconv.Itoa(rate))
}

// PlaybackFileCommand creates a cras_test_client playback-from-file command.
func PlaybackFileCommand(ctx context.Context, file string, duration, channels, rate int) *testexec.Cmd {
	return getCommand(ctx, playbackMode, file, duration, channels, getBlockSize(rate), rate)
}

// PlaybackCommand creates a cras_test_client playback command.
func PlaybackCommand(ctx context.Context, duration, blocksize int) *testexec.Cmd {
	return getCommand(ctx, playbackMode, "/dev/zero", duration, 2, blocksize, 48000)
}

// CaptureFileCommand creates a cras_test_client capture-to-file command.
func CaptureFileCommand(ctx context.Context, file string, duration, channels, rate int) *testexec.Cmd {
	return getCommand(ctx, captureMode, file, duration, channels, getBlockSize(rate), rate)
}

// CaptureCommand creates a cras_test_client capture command.
func CaptureCommand(ctx context.Context, duration, blocksize int) *testexec.Cmd {
	return getCommand(ctx, captureMode, "/dev/null", duration, 2, blocksize, 48000)
}
