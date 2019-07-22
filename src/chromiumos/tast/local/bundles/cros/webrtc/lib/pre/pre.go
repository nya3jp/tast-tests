// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pre provides Chrome Preconditions shared among camera tests.
package pre

import (
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// ChromeCameraPerf returns a precondition that Chrome is started with camera tests-specific
// setting and without verbose logging that can affect the performance.
// This precondition should be used only used for performance tests.
func ChromeCameraPerf() testing.Precondition { return chromeCameraPerfPre }

var chromeCameraPerfPre = chrome.NewPrecondition("camera_perf",
	chrome.ExtraArgs(
		// Avoid the need to grant camera/microphone permissions.
		"--use-fake-ui-for-media-stream"))
