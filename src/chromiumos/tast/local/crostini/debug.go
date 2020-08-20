// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

const (
	// FastDebug controls whether the precondition pre.Installed() attempts
	// to re-use an existing VM image instead of forcefully installing a
	// known good image each time.
	//
	// This option exists to speed up debugging on developer workstations.
	// It is not intended for use on lab devices for it may try to re-use
	// a contaminated VM left behind by a previous test.
	FastDebug = false
)
