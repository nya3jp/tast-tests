// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	//	"chromiumos/tast/local/chrome/nearbyshare/nearbysetup"
	//	"chromiumos/tast/local/chrome/nearbyshare/nearbytestutils"
	"chromiumos/tast/testing"
)

// resetTimeout is the timeout duration to trying reset of the current fixture.
const resetTimeout = 30 * time.Second

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "nearbyShareEnabledDataOfflineAllContacts",
		Desc:            "Nearby Share enabled and configured with 'Data Usage' set to 'Offline' and 'Visibility' set to 'All Contacts'",
		Impl:            &dataOfflineAllContactsFixture{},
		SetUpTimeout:    2 * time.Minute,
		TearDownTimeout: resetTimeout,
	})
}

type dataOfflineAllContactsFixture struct {
	cr *chrome.Chrome
}

// FixtData holds information made available to tests that specify this Fixture.
type FixtData struct {
	// Chrome is the running chrome instance.
	Chrome *chrome.Chrome

	// TestConn is a connection to the test extension.
	TestConn *chrome.TestConn

	// DeviceName is the device name configured for Nearby Share.
	DeviceName string
}

func (f *dataOfflineAllContactsFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	cr, err := chrome.New(
		ctx,
		chrome.EnableFeatures("IntentHandlingSharing", "NearbySharing", "Sharesheet"),
		chrome.ExtraArgs("--nearby-share-verbose-logging"),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	chrome.Lock()
	f.cr = cr
	/*
		tconn, err := f.cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Creating test API connection failed: ", err)
		}

		// Set up Nearby Share on the CrOS device.
		const crosBaseName = "cros_test"
		crosDisplayName := nearbytestutils.RandomDeviceName(crosBaseName)
		if err := nearbysetup.CrOSSetup(ctx, tconn, f.cr, nearbysetup.DataUsageOffline, nearbysetup.VisibilityAllContacts, crosDisplayName); err != nil {
			s.Fatal("Failed to set up Nearby Share: ", err)
		}

		return &FixtData{
			Chrome:     f.cr,
			TestConn:   tconn,
			DeviceName: crosDisplayName,
		}
	*/
	return cr
}

func (f *dataOfflineAllContactsFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	f.cr = nil
}

func (f *dataOfflineAllContactsFixture) Reset(ctx context.Context) error {
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}
	return nil
}

func (f *dataOfflineAllContactsFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *dataOfflineAllContactsFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}
