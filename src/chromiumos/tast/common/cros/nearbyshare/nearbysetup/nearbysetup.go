// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbysetup is used to set up the environment for Nearby Share tests.
package nearbysetup

import (
	"chromiumos/tast/common/cros/crossdevice"
)

// CrosAttributes contains information about the CrOS device that are relevant to Nearby Share.
type CrosAttributes struct {
	BasicAttributes *crossdevice.CrosAttributes
	DisplayName     string
	DataUsage       string
	Visibility      string
}
