// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyfixture

import (
	"time"

	nearbycommon "chromiumos/tast/common/cros/nearbyshare"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysnippet"
	"chromiumos/tast/testing"
)

// addARCFixtures registers fixtures for tests that initiate sharing from the ARC sharesheet.
func addARCFixtures() {
	testing.AddFixture(&testing.Fixture{
		Name:   "nearbyShareDataUsageOnlineNoOneARCEnabled",
		Desc:   "Nearby Share enabled on CrOS and Android configured with 'Data Usage' set to 'Online', 'Visibility' set to 'No One', and ARC enabled",
		Parent: "nearbyShareGAIALoginARCEnabled",
		Impl: NewNearbyShareFixture(fixtureOptions{
			crosDataUsage:              nearbycommon.DataUsageOnline,
			crosVisibility:             nearbycommon.VisibilityNoOne,
			androidDataUsage:           nearbysnippet.DataUsageOnline,
			androidVisibility:          nearbysnippet.VisibilityNoOne,
			crosSelectAndroidAsContact: false,
		}),
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"arc-app-dev@google.com",
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: postTestTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareDataUsageOfflineNoOneARCEnabled",
		Desc: "Nearby Share enabled on CrOS and Android configured with 'Data Usage' set to 'Offline', 'Visibility' set to 'No One', and ARC enabled",
		Impl: NewNearbyShareFixture(fixtureOptions{
			crosDataUsage:              nearbycommon.DataUsageOffline,
			crosVisibility:             nearbycommon.VisibilityNoOne,
			androidDataUsage:           nearbysnippet.DataUsageOffline,
			androidVisibility:          nearbysnippet.VisibilityNoOne,
			crosSelectAndroidAsContact: false,
		}),
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"arc-app-dev@google.com",
		},
		Parent:          "nearbyShareGAIALoginARCEnabled",
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: postTestTimeout,
	})
}
