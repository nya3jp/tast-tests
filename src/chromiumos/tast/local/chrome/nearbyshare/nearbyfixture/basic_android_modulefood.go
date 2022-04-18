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

// addModulefoodAndroidFixtures registers fixtures for basic CrOS<->Android sharing tests.
// The Android phone will be signed in with a GAIA account that is a member of the Nearby modulefood group,
// so it will receive the modulefood pre-release version of Android Nearby Share.
// These are the primary fixtures for the Nearby tests that we run across all CrOS channels.
func addModulefoodAndroidFixtures() {
	// Fixtures for in-contacts sharing tests.
	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareDataUsageOfflineAllContacts",
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
		Parent:          "nearbyShareGAIALoginAndroidAccount",
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:   "nearbyShareDataUsageOnlineAllContacts",
		Desc:   "Nearby Share enabled on CrOS and Android configured with 'Data Usage' set to 'Online',  'Visibility' set to 'All Contacts'",
		Parent: "nearbyShareGAIALoginAndroidAccount",
		Impl: NewNearbyShareFixture(fixtureOptions{
			crosDataUsage:              nearbycommon.DataUsageOnline,
			crosVisibility:             nearbycommon.VisibilityAllContacts,
			androidDataUsage:           nearbysnippet.DataUsageOnline,
			androidVisibility:          nearbysnippet.VisibilityAllContacts,
			crosSelectAndroidAsContact: false,
		}),
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	// Fixtures for high-visibility sharing tests.
	testing.AddFixture(&testing.Fixture{
		Name:   "nearbyShareDataUsageOnlineNoOne",
		Desc:   "Nearby Share enabled on CrOS and Android configured with 'Data Usage' set to 'Online' and 'Visibility' set to 'No One'",
		Parent: "nearbyShareGAIALogin",
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
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareDataUsageOfflineNoOne",
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
		Parent:          "nearbyShareGAIALogin",
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	// Fixtures for "Some contacts" visibility tests, that select which contacts to be visible to.
	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareDataUsageOfflineSomeContactsAndroidSelectedContact",
		Desc: "Nearby Share enabled on CrOS and Android with 'Data Usage' set to 'Offline' on both. The Android device 'Visibility' is 'All Contacts'. The CrOS device 'Visibility' is 'Some contacts' with the Android user set as an allowed contact so it will be visible to the Android device. The CrOS device is logged in with a GAIA account which is mutual contacts with the Android GAIA account",
		Impl: NewNearbyShareFixture(fixtureOptions{
			crosDataUsage:              nearbycommon.DataUsageOffline,
			crosVisibility:             nearbycommon.VisibilitySelectedContacts,
			androidDataUsage:           nearbysnippet.DataUsageOffline,
			androidVisibility:          nearbysnippet.VisibilityAllContacts,
			crosSelectAndroidAsContact: true,
		}),
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Parent: "nearbyShareGAIALogin",
		Vars: []string{
			customAndroidUsername,
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:   "nearbyShareDataUsageOnlineSomeContactsAndroidSelectedContact",
		Desc:   "Nearby Share enabled on CrOS and Android with 'Data Usage' set to 'Online' on both. The Android device 'Visibility' is 'All Contacts'. The CrOS device 'Visibility' is 'Some contacts' with the Android user set as an allowed contact so it will be visible to the Android device. The CrOS device is logged in with a GAIA account which is mutual contacts with the Android GAIA account",
		Parent: "nearbyShareGAIALogin",
		Impl: NewNearbyShareFixture(fixtureOptions{
			crosDataUsage:              nearbycommon.DataUsageOnline,
			crosVisibility:             nearbycommon.VisibilitySelectedContacts,
			androidDataUsage:           nearbysnippet.DataUsageOnline,
			androidVisibility:          nearbysnippet.VisibilityAllContacts,
			crosSelectAndroidAsContact: true,
		}),
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Vars: []string{
			customAndroidUsername,
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:   "nearbyShareDataUsageOfflineSomeContactsAndroidNotSelectedContact",
		Desc:   "Nearby Share enabled on CrOS and Android with 'Data Usage' set to 'Offline' on both. The Android device 'Visibility' is 'All Contacts'. The CrOS device 'Visibility' is 'Some contacts' with no contacts selected, so it will not be visible to the Android device. The CrOS device is logged in with a GAIA account which is mutual contacts with the Android GAIA account",
		Parent: "nearbyShareGAIALogin",
		Impl: NewNearbyShareFixture(fixtureOptions{
			crosDataUsage:              nearbycommon.DataUsageOffline,
			crosVisibility:             nearbycommon.VisibilitySelectedContacts,
			androidDataUsage:           nearbysnippet.DataUsageOffline,
			androidVisibility:          nearbysnippet.VisibilityAllContacts,
			crosSelectAndroidAsContact: false,
		}),
		Vars: []string{
			customAndroidUsername,
		},
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
}
