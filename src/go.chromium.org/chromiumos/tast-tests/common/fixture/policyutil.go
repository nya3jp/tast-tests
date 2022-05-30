// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixture

// Fixture defined in go.chromium.org/chromiumos/tast-tests/remote/policyutil/enrolled_fixture.go.
const (
	// Enrolled is a fixture name.
	Enrolled = "enrolled"
)

// Fixtures defined in go.chromium.org/chromiumos/tast-tests/local/policyutil/fixtures/fakedms.go.
const (
	// FakeDMS is a fixture name.
	FakeDMS = "fakeDMS"
	// FakeDMSEnrolled is a fixture name.
	FakeDMSEnrolled = "fakeDMSEnrolled"
)

// Fixtures defined in go.chromium.org/chromiumos/tast-tests/local/policyutil/fixtures/chrome.go.
const (
	// ChromePolicyLoggedIn is a fixture name.
	ChromePolicyLoggedIn = "chromePolicyLoggedIn"
	// ChromePolicyLoggedInWithoutPersonalizationHub is a fixture name.
	ChromePolicyLoggedInWithoutPersonalizationHub = "chromePolicyLoggedInWithoutPersonalizationHub"
	// ChromePolicyLoggedInLockscreen is a fixture name.
	ChromePolicyLoggedInLockscreen = "chromePolicyLoggedInLockscreen"
	// ChromePolicyLoggedInIsolatedApp is a fixture name.
	ChromePolicyLoggedInIsolatedApp = "chromePolicyLoggedInIsolatedApp"
	// ChromePolicyLoggedInFeatureJourneys is a fixture name.
	ChromePolicyLoggedInFeatureJourneys = "chromePolicyLoggedInFeatureJourneys"
	// ChromePolicyLoggedInFeatureChromeLabs is a fixture name.
	ChromePolicyLoggedInFeatureChromeLabs = "chromePolicyLoggedInFeatureChromeLabs"
	// ChromeEnrolledLoggedIn is a fixture name.
	ChromeEnrolledLoggedIn = "chromeEnrolledLoggedIn"
	// ChromeEnrolledLoggedInARC is a fixture name.
	ChromeEnrolledLoggedInARC = "chromeEnrolledLoggedInARC"
	// ChromeAdminDeskTemplatesLoggedIn is a fixture name.
	ChromeAdminDeskTemplatesLoggedIn = "chromeAdminDeskTemplatesLoggedIn"
)

// Fixtures defined in go.chromium.org/chromiumos/tast-tests/local/mgs/fixture.go.
const (
	ManagedGuestSession               = "managedGuestSession"
	ManagedGuestSessionWithExtensions = "managedGuestSessionWithExtensions"
)

// Fixtures defined in go.chromium.org/chromiumos/tast-tests/local/policyutil/fixtures/lacros.go.
const (
	// LacrosPolicyLoggedIn is a fixture name.
	LacrosPolicyLoggedIn = "lacrosPolicyLoggedIn"
	// LacrosPolicyLoggedInFeatureJourneys is a fixture name.
	LacrosPolicyLoggedInFeatureJourneys = "lacrosPolicyLoggedInFeatureJourneys"
	// LacrosPolicyLoggedInFeatureChromeLabs is a fixture name.
	LacrosPolicyLoggedInFeatureChromeLabs = "lacrosPolicyLoggedInFeatureChromeLabs"
	// LacrosPolicyLoggedInRealUser is a fixture name.
	LacrosPolicyLoggedInRealUser = "lacrosPolicyLoggedInRealUser"
)

// Fixtures defined in go.chromium.org/chromiumos/tast-tests/local/policyutil/fixtures/persistent.go.
const (
	// PersistentLacros is a fixture name.
	PersistentLacros = "persistentLacros"
	// PersistentLacrosRealUser is a fixture name.
	PersistentLacrosRealUser = "persistentLacrosRealUser"
	// PersistentFamilyLink is a fixture name.
	PersistentFamilyLink = "persistentFamilyLink"
	// PersistentFamilyLinkARC is a fixture name.
	PersistentFamilyLinkARC = "persistentFamilyLinkARC"
)
