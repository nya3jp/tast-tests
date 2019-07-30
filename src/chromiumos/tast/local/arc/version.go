// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/lsbrelease"
)

const (
	// SDKN is the SDK version of Android N.
	SDKN = 25

	// SDKP is the SDK version of Android P.
	SDKP = 28

	// SDKQ is the SDK version of Android Q.
	SDKQ = 29
)

// SDKVersion returns the ARC's Android SDK version for the current ARC image
// installed into the DUT.
func SDKVersion() (int, error) {
	m, err := lsbrelease.Load()
	if err != nil {
		return 0, err
	}
	val, ok := m[lsbrelease.ARCSDKVersion]
	if !ok {
		return 0, errors.Errorf("failed to find %s in /etc/lsb-release", lsbrelease.ARCSDKVersion)
	}
	ret, err := strconv.Atoi(val)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse SDK version %q", val)
	}
	return ret, nil
}
