// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyfixture

import (
	"time"

	nearbycommon "chromiumos/tast/common/cros/nearbyshare"
	"chromiumos/tast/local/chrome/crossdevice"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysnippet"
	"chromiumos/tast/testing"
)

// addDevAndroidFixtures registers fixtures for basic CrOS<->Android sharing tests.
// The Android phone will be signed in with a GAIA account that is a member of the Nearby dev user group,
// so it will be using the dev Nearby version.
func addDevAndroidFixtures() {
	// Fixtures for in-contacts sharing tests.
	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareDataUsageOfflineAllContactsDev",
		Desc: "Nearby Share enabled on CrOS and Android configured with 'Data Usage' set to 'Offline',  'Visibility' set to 'All Contacts'",
		Impl: NewNearbyShareFixture(fixtureOptions{
			crosDataUsage:              nearbycommon.DataUsageOffline,
			crosVisibility:             nearbycommon.VisibilityAllContacts,
			androidDataUsage:           nearbysnippet.DataUsageOffline,
			androidVisibility:          nearbysnippet.VisibilityAllContacts,
			crosSelectAndroidAsContact: false,
		}),
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Parent:          "nearbyShareGAIALoginAndroidAccountDev",
		SetUpTimeout:    3*time.Minute + crossdevice.BugReportDuration,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: postTestTimeout,
	})

	// Fixtures for high-visibility sharing tests.
	testing.AddFixture(&testing.Fixture{
		Name:   "nearbyShareDataUsageOnlineNoOneDev",
		Desc:   "Nearby Share enabled on CrOS and Android configured with 'Data Usage' set to 'Online' and 'Visibility' set to 'No One'",
		Parent: "nearbyShareGAIALoginDev",
		Impl: NewNearbyShareFixture(fixtureOptions{
			crosDataUsage:              nearbycommon.DataUsageOnline,
			crosVisibility:             nearbycommon.VisibilityNoOne,
			androidDataUsage:           nearbysnippet.DataUsageOnline,
			androidVisibility:          nearbysnippet.VisibilityNoOne,
			crosSelectAndroidAsContact: false,
		}),
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		SetUpTimeout:    3*time.Minute + crossdevice.BugReportDuration,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: postTestTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareDataUsageOfflineNoOneDev",
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
		Parent:          "nearbyShareGAIALoginDev",
		SetUpTimeout:    3*time.Minute + crossdevice.BugReportDuration,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: postTestTimeout,
	})
}
