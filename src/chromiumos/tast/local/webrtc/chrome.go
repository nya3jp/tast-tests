// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webrtc

import "chromiumos/tast/local/media/logging"

// This file provides utility variables and functions for WebRTC related tests.

// ChromeArgsWithFakeCameraInput returns Chrome extra args as string slice for
// video test with a Fake WebCam (a.k.a. "rolling pacman") s live camera input.
// If verbose is true, it appends extra args for verbose logging.
func ChromeArgsWithFakeCameraInput(verbose bool) []string {
	args := []string{
		// See https://webrtc.org/testing/
		// Feed a test pattern to getUserMedia() instead of live camera input.
		"--use-fake-device-for-media-stream",
		// Avoid the need to grant camera/microphone permissions.
		"--use-fake-ui-for-media-stream",
		// Disable the autoplay policy not to be affected by actions from outside of tests.
		// cf. https://developers.google.com/web/updates/2017/09/autoplay-policy-changes
		"--autoplay-policy=no-user-gesture-required",
	}
	if verbose {
		args = append(args, logging.ChromeVmoduleFlag())
	}
	return args
}

// ChromeArgsWithFileCameraInput returns Chrome extra args as string slice
// for video test with a Y4M/MJPEG fileName streamed as live camera input.
// If verbose is true, it appends extra args for verbose logging.
// NOTE(crbug.com/955079): performance test should unset verbose.
func ChromeArgsWithFileCameraInput(fileName string, verbose bool) []string {
	args := []string{
		// Feed a Y4M test file to getUserMedia() instead of live camera input.
		"--use-file-for-fake-video-capture=" + fileName,
	}
	args = append(ChromeArgsWithFakeCameraInput(verbose), args...)
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
