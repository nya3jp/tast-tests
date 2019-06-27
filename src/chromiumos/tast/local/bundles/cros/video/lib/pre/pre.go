// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pre provides Chrome Preconditions shared among video tests.
// Although right now no test depends on pre package, it is going to be used
// when we want to promote browser dependent tests to CQ as we need to
// eliminate unnecessary Chrome restart.
// TODO(crbug.com/976592): Use ChromeVideo() when promoting tests to CQ.
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

var chromeVideoPre = chrome.NewPrecondition("video", chromeargs.Default)
