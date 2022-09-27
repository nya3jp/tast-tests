// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package quickanswers contains helper functions for the local Tast tests
// that exercise ChromeOS Quick answers feature.
package quickanswers

import (
	"context"

	"chromiumos/tast/local/chrome"
)

// SetPrefValue is a helper function to set value for Quick answers related prefs.
// Note that the pref needs to be allowlisted here:
// https://cs.chromium.org/chromium/src/chrome/browser/extensions/api/settings_private/prefs_util.cc
func SetPrefValue(ctx context.Context, tconn *chrome.TestConn, prefName string, value interface{}) error {
	return tconn.Call(ctx, nil, `tast.promisify(chrome.settingsPrivate.setPref)`, prefName, value)
}
