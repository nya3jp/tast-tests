// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pre provides Chrome Preconditions shared among video tests.
package pre

import (
	"strings"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// ChromeVideo returns a precondition that Chrome is started with video tests-specific
// flags and is already logged in when a test is run.
func ChromeVideo() testing.Precondition { return chromeVideoPre }

var chromeVideoPre = chrome.NewPrecondition("video",
	chrome.ExtraArgs(
		// Enable verbose log messages for video components.
		"--vmodule="+strings.Join([]string{
			"*/media/gpu/*video_decode_accelerator.cc=2",
			"*/media/gpu/*video_encode_accelerator.cc=2",
			"*/media/gpu/*jpeg_decode_accelerator.cc=2",
			"*/media/gpu/*jpeg_encode_accelerator.cc=2",
			"*/media/gpu/*image_processor.cc=2",
			"*/media/gpu/*v4l2_device.cc=2"}, ","),
		// Disable the autoplay policy not to be affected by actions from outside of tests.
		// cf. https://developers.google.com/web/updates/2017/09/autoplay-policy-changes
		"--autoplay-policy=no-user-gesture-required",
		// Avoid the need to grant camera/microphone permissions.
		"--use-fake-ui-for-media-stream"))
