// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fixtvals provides common types for fixtures of the enterpriseconnectors package.
package fixtvals

import (
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
)

// PolicyParams entail parameters describing the set policy for a user.
type PolicyParams struct {
	AllowsImmediateDelivery bool // specifies whether immediate delivery of files is allowed
	AllowsUnscannableFiles  bool // specifies whether unscannable files (large or encrypted) are allowed
	ScansEnabledForDownload bool // specifies whether malware and dlp scans are enabled for download
	ScansEnabledForUpload   bool // specifies whether malware and dlp scans are enabled for upload
}

// FixtValue is a type that embeds both PolicyParams and lacrosfixt.FixtValue
type FixtValue struct {
	PolicyParams
	lacrosfixt.FixtValue
}
