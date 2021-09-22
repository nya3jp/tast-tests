// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixture

import (
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
)

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
	// FakeDMSFamilyLink is a fixture name.
	FakeDMSFamilyLink = "fakeDMSFamilyLink"
	// FakeDMSFamilyLinkArc is a fixture name.
	FakeDMSFamilyLinkArc = "fakeDMSFamilyLinkArc"
)

// Fixtures defined in chromiumos/tast/local/policyutil/fixtures/chrome.go.
const (
	// ChromePolicyLoggedIn is a fixture name.
	ChromePolicyLoggedIn = "chromePolicyLoggedIn"
	// ChromeEnrolledLoggedIn is a fixture name.
	ChromeEnrolledLoggedIn = "chromeEnrolledLoggedIn"
	// ChromeEnrolledLoggedInARC is a fixture name.
	ChromeEnrolledLoggedInARC = "chromeEnrolledLoggedInARC"
)

// HasChrome is an interface to get Chrome from a fixture.
type HasChrome interface {
	Chrome() *chrome.Chrome
}

// HasFakeDMS is an interface to get FakeDMS from a fixture.
type HasFakeDMS interface {
	FakeDMS() *fakedms.FakeDMS
}
