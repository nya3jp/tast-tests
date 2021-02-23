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

// NewNearbyShareFixture creates a new implementation of the Nearby Share fixture.
func NewNearbyShareFixture(dataUsage nearbysetup.DataUsage, visibility nearbysetup.Visibility, gaiaLogin bool, opts ...chrome.Option) testing.FixtureImpl {
	defaultNearbyOpts := []chrome.Option{
		chrome.EnableFeatures("IntentHandlingSharing", "NearbySharing", "Sharesheet"),
		chrome.ExtraArgs("--nearby-share-verbose-logging"),
	}
	return &nearbyShareFixture{
		opts:       append(defaultNearbyOpts, opts...),
		dataUsage:  dataUsage,
		visibility: visibility,
		gaiaLogin:  gaiaLogin,
	}
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "nearbyShareDataUsageOfflineAllContactsTestUser",
		Desc:            "Nearby Share enabled and configured with 'Data Usage' set to 'Offline' and 'Visibility' set to 'All Contacts'",
		Impl:            NewNearbyShareFixture(nearbysetup.DataUsageOffline, nearbysetup.VisibilityAllContacts, false),
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareDataUsageOfflineAllContactsGAIA",
		Desc: "Nearby Share enabled and configured with 'Data Usage' set to 'Offline',  'Visibility' set to 'All Contacts' and logged in with a real GAIA account",
		Impl: NewNearbyShareFixture(nearbysetup.DataUsageOffline, nearbysetup.VisibilityAllContacts, true),
		Vars: []string{
			"username",
			"password",
			"nearbyshare.cros_username",
			"nearbyshare.cros_password",
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
}

type nearbyShareFixture struct {
	cr         *chrome.Chrome
	opts       []chrome.Option
	dataUsage  nearbysetup.DataUsage
	visibility nearbysetup.Visibility
	gaiaLogin  bool

	// ChromeReader is the line reader for collecting Chrome logs.
	ChromeReader *syslog.LineReader
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

func (f *nearbyShareFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if f.gaiaLogin {
		username := s.RequiredVar("nearbyshare.cros_username")
		password := s.RequiredVar("nearbyshare.cros_password")
		customUser, userOk := s.Var("username")
		customPass, passOk := s.Var("password")
		if userOk && passOk {
			s.Log("Logging in with user-provided credentials")
			username = customUser
			password = customPass
		} else {
			s.Log("Logging in with default GAIA credentials")
		}
		f.opts = append(f.opts, chrome.Auth(username, password, ""), chrome.GAIALogin())
	}

	cr, err := chrome.New(
		ctx,
		f.opts...,
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
	if err := nearbysetup.CrOSSetup(ctx, tconn, cr, f.dataUsage, f.visibility, crosDisplayName); err != nil {
		s.Fatal("Failed to set up Nearby Share: ", err)
	}

	f.cr = cr
	return &FixtData{
		Chrome:     cr,
		TestConn:   tconn,
		DeviceName: crosDisplayName,
	}
}

func (f *nearbyShareFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	f.cr = nil
}

func (f *nearbyShareFixture) Reset(ctx context.Context) error {
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}
	return nil
}

func (f *nearbyShareFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	chromeReader, err := nearbytestutils.StartLogging(ctx, syslog.ChromeLogFile)
	if err != nil {
		s.Fatal("Failed to start Chrome logging: ", err)
	}
	f.ChromeReader = chromeReader
}

func (f *nearbyShareFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if f.ChromeReader == nil {
		s.Fatal("ChromeReader not defined")
	}
	if err := nearbytestutils.SaveLogs(ctx, f.ChromeReader, filepath.Join(s.OutDir(), ChromeLog)); err != nil {
		s.Fatal("Failed to save Chrome log: ", err)
	}
}
