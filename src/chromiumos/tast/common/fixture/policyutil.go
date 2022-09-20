// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixture

// Fixture defined in chromiumos/tast/remote/policyutil/enrolled_fixture.go.
const (
	// Enrolled is a fixture name.
	Enrolled = "enrolled"
)

// Fixtures defined in chromiumos/tast/local/policyutil/fixtures/fakedms.go.
const (
	// FakeDMS is a fixture name.
	FakeDMS = "fakeDMS"
	// FakeDMSEnrolled is a fixture name.
	FakeDMSEnrolled = "fakeDMSEnrolled"
)

// Fixtures defined in chromiumos/tast/local/policyutil/fixtures/chrome.go.
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

// Fixtures defined in chromiumos/tast/local/mgs/fixture.go.
const (
	ManagedGuestSession               = "managedGuestSession"
	ManagedGuestSessionWithExtensions = "managedGuestSessionWithExtensions"
)

// Fixtures defined in chromiumos/tast/local/policyutil/fixtures/lacros.go.
const (
	// LacrosPolicyLoggedIn is a fixture name.
	LacrosPolicyLoggedIn = "lacrosPolicyLoggedIn"
	// LacrosPrimaryPolicyLoggedIn is a fixture name.
	LacrosPrimaryPolicyLoggedIn = "lacrosPrimaryPolicyLoggedIn"
	// LacrosPolicyLoggedInFeatureJourneys is a fixture name.
	LacrosPolicyLoggedInFeatureJourneys = "lacrosPolicyLoggedInFeatureJourneys"
	// LacrosPolicyLoggedInFeatureChromeLabs is a fixture name.
	LacrosPolicyLoggedInFeatureChromeLabs = "lacrosPolicyLoggedInFeatureChromeLabs"
	// LacrosPolicyLoggedInRealUser is a fixture name.
	LacrosPolicyLoggedInRealUser = "lacrosPolicyLoggedInRealUser"
	// LacrosAdminDeskTemplatesLoggedIn is a fixture name.
	LacrosAdminDeskTemplatesLoggedIn = "lacrosAdminDeskTemplatesLoggedIn"
)

// Fixtures defined in chromiumos/tast/local/policyutil/fixtures/persistent.go.
const (
	// PersistentLacros is a fixture name.
	PersistentLacros = "persistentLacros"
	// PersistentLacrosPrimary is a fixture name.
	PersistentLacrosPrimary = "persistentLacrosPrimary"
	// PersistentLacrosEnrolled is a fixture name.
	PersistentLacrosEnrolled = "persistentLacrosEnrolled"
	// PersistentLacrosRealUser is a fixture name.
	PersistentLacrosRealUser = "persistentLacrosRealUser"
	// PersistentFamilyLink is a fixture name.
	PersistentFamilyLink = "persistentFamilyLink"
	// PersistentFamilyLinkARC is a fixture name.
	PersistentFamilyLinkARC = "persistentFamilyLinkARC"
	// PersistentProjectorEDU is a fixture name.
	PersistentProjectorEDU = "persistentProjectorEDU"
)
