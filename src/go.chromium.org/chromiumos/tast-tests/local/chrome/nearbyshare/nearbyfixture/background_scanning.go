// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyfixture

import (
	"time"

	nearbycommon "go.chromium.org/chromiumos/tast-tests/common/cros/nearbyshare"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/nearbyshare/nearbysnippet"
	"go.chromium.org/chromiumos/tast/testing"
)

// addBackgroundScanningFixtures registers fixtures for tests with background scanning enabled.
func addBackgroundScanningFixtures() {
	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareDataUsageOfflineNoOneBackgroundScanningEnabled",
		Desc: "Nearby Share enabled on CrOS and Android configured with 'Data Usage' set to 'Offline' and 'Visibility' set to 'No One'",
		Impl: NewNearbyShareFixture(fixtureOptions{
			crosDataUsage:              nearbycommon.DataUsageOffline,
			crosVisibility:             nearbycommon.VisibilityNoOne,
			androidDataUsage:           nearbysnippet.DataUsageOffline,
			androidVisibility:          nearbysnippet.VisibilityNoOne,
			crosSelectAndroidAsContact: false,
		}),
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Parent:          "nearbyShareGAIALoginBackgroundScanningEnabled",
		SetUpTimeout:    3 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
}
