// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysetup"
	"chromiumos/tast/local/chrome/nearbyshare/nearbytestutils"
	"chromiumos/tast/local/syslog"
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
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
}

type dataOfflineAllContactsFixture struct {
	cr *chrome.Chrome

	// ChromeReader is the line reader for collecting Chrome logs.
	ChromeReader *syslog.LineReader

	// MessagesReader is the line reader for collecting Messages logs.
	MessageReader *syslog.LineReader
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

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Set up Nearby Share on the CrOS device.
	const crosBaseName = "cros_test"
	crosDisplayName := nearbytestutils.RandomDeviceName(crosBaseName)
	if err := nearbysetup.CrOSSetup(ctx, tconn, cr, nearbysetup.DataUsageOffline, nearbysetup.VisibilityAllContacts, crosDisplayName); err != nil {
		s.Fatal("Failed to set up Nearby Share: ", err)
	}

	f.cr = cr
	return &FixtData{
		Chrome:     cr,
		TestConn:   tconn,
		DeviceName: crosDisplayName,
	}
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

func (f *dataOfflineAllContactsFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	chromeReader, err := nearbytestutils.StartLogging(ctx, syslog.ChromeLogFile)
	if err != nil {
		s.Fatal("Failed to start Chrome logging: ", err)
	}
	f.ChromeReader = chromeReader

	messageReader, err := nearbytestutils.StartLogging(ctx, syslog.MessageFile)
	if err != nil {
		s.Fatal("Failed to start message logging: ", err)
	}
	f.MessageReader = messageReader
}

func (f *dataOfflineAllContactsFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if f.ChromeReader == nil {
		s.Fatal("ChromeReader not defined")
	}
	if err := nearbytestutils.SaveLogs(ctx, f.ChromeReader, filepath.Join(s.OutDir(), ChromeLog)); err != nil {
		s.Fatal("Failed to save Chrome log: ", err)
	}

	if f.MessageReader == nil {
		s.Fatal("MessageReader not defined")
	}
	if err := nearbytestutils.SaveLogs(ctx, f.MessageReader, filepath.Join(s.OutDir(), MessageLog)); err != nil {
		s.Fatal("Failed to save Message log: ", err)
	}
}
