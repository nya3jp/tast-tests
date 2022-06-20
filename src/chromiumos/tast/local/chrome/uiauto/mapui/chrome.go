// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mapui

import "chromiumos/tast/testing"

var chromeHomeButton = testing.RegisterVarString(
	"mapui.chrome.home_button",
	`{ "name": "Home", "role": "button"}`,
	"UI Node for chrome address bar",
)

// ChromeHomeButton is a node finder for the Home button.
var ChromeHomeButton = nodeFromStringVar(chromeHomeButton)
