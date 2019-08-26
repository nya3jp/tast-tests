// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import "chromiumos/tast/local/media/logging"

// This file provides utility variables and functions for webrtc.* tests.

const (
	// LoopbackPage is a webpage for WebRTC loopback test.
	LoopbackPage = "loopback.html"
	// AddStatsJSFile is a JavaScript file for replacing addLegacyStats() in chrome://webrtc-internals.
	AddStatsJSFile = "add_stats.js"
)

// ChromeArgsWithCameraInput returns Chrome extra args as string slice
// for video test with Y4M stream file as live camera input.
// If verbose is true, it appends extra args for verbose logging.
// NOTE(crbug.com/955079): performance test should unset verbose.
func ChromeArgsWithCameraInput(stream string, verbose bool) []string {
	args := []string{
		// See https://webrtc.org/testing/
		// Feed a test pattern to getUserMedia() instead of live camera input.
		"--use-fake-device-for-media-stream",
		// Avoid the need to grant camera/microphone permissions.
		"--use-fake-ui-for-media-stream",
		// Feed a Y4M test file to getUserMedia() instead of live camera input.
		"--use-file-for-fake-video-capture=" + stream,
		// Disable the autoplay policy not to be affected by actions from outside of tests.
		// cf. https://developers.google.com/web/updates/2017/09/autoplay-policy-changes
		"--autoplay-policy=no-user-gesture-required",
	}
	if verbose {
		args = append(args, logging.ChromeVmoduleFlag())
	}
	return args
}

// DataFiles returns a list of required files that tests that use this package
// should include in their Data fields.
func DataFiles() []string {
	return []string{
		"third_party/blackframe.js",
		"third_party/munge_sdp.js",
		"third_party/ssim.js",
	}
}

// LoopbackDataFiles returns a list of required files for opening WebRTC loopback test page.
func LoopbackDataFiles() []string {
	return append(DataFiles(), LoopbackPage)
}
