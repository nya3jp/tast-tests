// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chromeargs provides common Chrome args used for video tests.
package chromeargs

import (
	"chromiumos/tast/local/bundles/cros/video/lib/logging"
	"chromiumos/tast/local/chrome"
)

// Default is used to append arguments to start Chrome for video tests.
var Default = chrome.ExtraArgs(
	logging.ChromeVmoduleFlag(),
	// Disable the autoplay policy not to be affected by actions from outside of tests.
	// cf. https://developers.google.com/web/updates/2017/09/autoplay-policy-changes
	"--autoplay-policy=no-user-gesture-required",
	// Avoid the need to grant camera/microphone permissions.
	"--use-fake-ui-for-media-stream")
