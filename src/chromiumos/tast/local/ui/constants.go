// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ui provides common constants used for UI tests.
package ui

import "chromiumos/tast/local/chrome"

// PerftestURL specifies the URL to be used for the browser windows for
// performance tests of UI animation and smoothness. The empty URL (about:blank)
// should be avoided since it's too simple for performance test. The new tab
// page (chrome://newtab) would be great but it is not stable (e.g. the logo can
// be replaced on holidays).
const PerftestURL = chrome.VersionURL
