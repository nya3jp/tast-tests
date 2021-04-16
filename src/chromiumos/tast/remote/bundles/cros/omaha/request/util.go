// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package request

import "strings"

// MatchChromeVersion checks if a given version matches the provided major version.
func MatchChromeVersion(major, version string) bool {
	return strings.HasPrefix(version, major+".")
}

// MatchChromeOSVersion checks if a given version matches the provided major version.
func MatchChromeOSVersion(major, version string) bool {
	return MatchChromeVersion(major, version)
}
