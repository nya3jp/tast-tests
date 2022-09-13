// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gaiaenrollment

// TestParams is a helper struct to keep parameters for gaia enrollment tests.
type TestParams struct {
	DMServer string // device management server url
	PoolID   string // poolID for the used test account
}
