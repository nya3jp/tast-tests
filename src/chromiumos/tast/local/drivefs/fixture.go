// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drivefs

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const driveFsSetupTimeout = time.Minute

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "driveFsStarted",
		Desc:            "Ensures DriveFS is mounted and provides an authenticated Drive API Client",
		Contacts:        []string{"benreich@chromium.org", "chromeos-files-syd@chromium.org"},
		Impl:            &fixture{},
		SetUpTimeout:    chrome.LoginTimeout + driveFsSetupTimeout,
		ResetTimeout:    driveFsSetupTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars: []string{
			"filemanager.drive_credentials",
			"filemanager.password",
			"filemanager.refresh_token",
			"filemanager.user",
		},
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "driveFsStartedWithNativeMessaging",
		Desc:     "Ensures DriveFS is mounted and the bidirectional messaging functionality is enabled",
		Contacts: []string{"austinct@chromium.org", "chromeos-files-syd@chromium.org"},
		Impl: &fixture{chromeOptions: []chrome.Option{
			chrome.EnableFeatures("DriveFsBidirectionalNativeMessaging"),
		}},
		SetUpTimeout:    chrome.LoginTimeout + driveFsSetupTimeout,
		ResetTimeout:    driveFsSetupTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars: []string{
			"filemanager.drive_credentials",
			"filemanager.password",
			"filemanager.refresh_token",
			"filemanager.user",
		},
	})
}

// FixtureData is the struct available for tests.
type FixtureData struct {
	// Chrome is a connection to an already-started Chrome instance.
	// It cannot be closed by tests.
	Chrome *chrome.Chrome

	// The path that DriveFS has mounted at.
	MountPath string

	// The API connection to the Test extension, reused by tests.
	TestAPIConn *chrome.TestConn

	// The APIClient singleton.
	APIClient *APIClient
}

type fixture struct {
	mountPath     string // The path where Drivefs is mounted
	cr            *chrome.Chrome
	tconn         *chrome.TestConn
	APIClient     *APIClient
	chromeOptions []chrome.Option
}

func (f *fixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	ctx, st := timing.Start(ctx, "prepare_drivefs_fixture")
	defer st.End()

	// If mountPath exists and API client is not nil, check if Drive has stabilized and return early if it has.
	if f.mountPath != "" && f.APIClient != nil {
		mountPath, err := WaitForDriveFs(ctx, f.cr.NormalizedUser())
		if err != nil {
			s.Log("Failed waiting for DriveFS to stabilize: ", err)
			chrome.Unlock()
			f.cleanUp(ctx, s)
		} else {
			f.mountPath = mountPath
			return &FixtureData{
				Chrome:      f.cr,
				MountPath:   f.mountPath,
				TestAPIConn: f.tconn,
				APIClient:   f.APIClient,
			}
		}
	}

	// If initialization fails, this defer is used to clean-up the partially-initialized pre.
	// Stolen verbatim from arc/pre.go
	shouldClose := true
	defer func() {
		if shouldClose {
			f.cleanUp(ctx, s)
		}
	}()

	func() {
		ctx, cancel := context.WithTimeout(ctx, chrome.LoginTimeout)
		defer cancel()

		username := s.RequiredVar("filemanager.user")
		password := s.RequiredVar("filemanager.password")
		var err error
		f.cr, err = chrome.New(ctx, append(f.chromeOptions, chrome.GAIALogin(chrome.Creds{User: username, Pass: password}), chrome.ARCDisabled())...)
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
	}()

	mountPath, err := WaitForDriveFs(ctx, f.cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed waiting for DriveFS to start: ", err)
	}
	s.Log("drivefs fully started")
	f.mountPath = mountPath

	tconn, err := f.cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed creating test API connection: ", err)
	}
	f.tconn = tconn

	jsonCredentials := s.RequiredVar("filemanager.drive_credentials")
	refreshToken := s.RequiredVar("filemanager.refresh_token")

	// Perform Drive API authentication.
	APIClient, err := CreateAPIClient(ctx, f.cr, jsonCredentials, refreshToken)
	if err != nil {
		s.Fatal("Failed creating a APIClient instance: ", err)
	}
	f.APIClient = APIClient

	// Lock Chrome and make sure deferred function does not run cleanup.
	chrome.Lock()
	shouldClose = false

	return &FixtureData{
		Chrome:      f.cr,
		MountPath:   f.mountPath,
		TestAPIConn: f.tconn,
		APIClient:   f.APIClient,
	}
}

// TearDown ensures the smb daemon is shutdown gracefully and all the temporary
// directories and files are cleaned up.
func (f *fixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()
	f.cleanUp(ctx, s)
}

// Reset unmounts any mounted SMB shares and removes all the contents of the
// guest share in between tests.
func (f *fixture) Reset(ctx context.Context) error {
	return nil
}

func (f *fixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *fixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

// cleanUp closes Chrome, resets the mountPath to empty string and sets tconn to nil.
func (f *fixture) cleanUp(ctx context.Context, s *testing.FixtState) {
	f.tconn = nil
	f.APIClient = nil
	f.mountPath = ""

	if f.cr != nil {
		if err := f.cr.Close(ctx); err != nil {
			s.Log("Failed closing chrome: ", err)
		}
		f.cr = nil
	}
}
