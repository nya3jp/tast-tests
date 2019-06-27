// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pre provides Chrome Preconditions shared among video tests.
package pre

import (
	"chromiumos/tast/local/bundles/cros/video/lib/chromeargs"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// ChromeVideo returns a precondition that Chrome is started with video tests-specific
// flags and is already logged in when a test is run.
// This precondition must not be used for performance tests, as verbose logging might might affect
// the performance.
func ChromeVideo() testing.Precondition { return chromeVideoPre }

var chromeVideoPre = chrome.NewPrecondition("video", chromeargs.DefaultArgs)
