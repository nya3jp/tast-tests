// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drivefs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const (
	driveFsSetupTimeout = time.Minute
)

var (
	driveAPIScopes = []string{"https://www.googleapis.com/auth/drive"}
)

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
			"drivefs.accountPool",
			"drivefs.extensionClientID",
		},
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "driveFsStartedTrashEnabled",
		Desc:            "Ensures DriveFS is mounted and provides an authenticated Drive API Client",
		Contacts:        []string{"benreich@chromium.org", "chromeos-files-syd@chromium.org"},
		Impl:            &fixture{chromeOptions: []chrome.Option{chrome.EnableFeatures("FilesTrash")}},
		SetUpTimeout:    chrome.LoginTimeout + driveFsSetupTimeout,
		ResetTimeout:    driveFsSetupTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars: []string{
			"drivefs.accountPool",
			"drivefs.extensionClientID",
		},
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "driveFsStartedWithNativeMessaging",
		Desc:     "Ensures DriveFS is mounted and the bidirectional messaging functionality is enabled",
		Contacts: []string{"austinct@chromium.org", "chromeos-files-syd@chromium.org"},
		Impl: &fixture{chromeOptions: []chrome.Option{
			chrome.EnableFeatures("DriveFsBidirectionalNativeMessaging"),
		}, drivefsOptions: map[string]string{
			"switchblade":     "true",
			"switchblade_dss": "true",
		}},
		SetUpTimeout:    chrome.LoginTimeout + driveFsSetupTimeout,
		ResetTimeout:    driveFsSetupTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars: []string{
			"drivefs.accountPool",
			"drivefs.extensionClientID",
		},
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "driveFsStartedWithChromeNetworking",
		Desc:     "Ensures DriveFS is mounted and the Chrome Network Service bridge is enabled",
		Contacts: []string{"travislane@google.com", "chromeos-files-syd@chromium.org"},
		Impl: &fixture{chromeOptions: []chrome.Option{
			chrome.EnableFeatures("DriveFsChromeNetworking"),
		}, drivefsOptions: map[string]string{
			"use_cros_http_client": "true",
		}},
		SetUpTimeout:    chrome.LoginTimeout + driveFsSetupTimeout,
		ResetTimeout:    driveFsSetupTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars: []string{
			"drivefs.accountPool",
			"drivefs.extensionClientID",
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

	// The DriveFS helper, reused by tests.
	DriveFs *DriveFs
}

type fixture struct {
	mountPath      string // The path where Drivefs is mounted
	cr             *chrome.Chrome
	tconn          *chrome.TestConn
	APIClient      *APIClient
	driveFs        *DriveFs
	chromeOptions  []chrome.Option
	drivefsOptions map[string]string
}

func (f *fixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	ctx, st := timing.Start(ctx, "prepare_drivefs_fixture")
	defer st.End()

	// If mountPath exists and API client is not nil, check if Drive has stabilized and return early if it has.
	if f.mountPath != "" && f.APIClient != nil {
		dfs, err := NewDriveFs(ctx, f.cr.NormalizedUser())
		if err != nil {
			s.Log("Failed waiting for DriveFS to stabilize: ", err)
			chrome.Unlock()
			f.cleanUp(ctx, s)
		} else {
			f.driveFs = dfs
			f.mountPath = f.driveFs.MountPath()
			return &FixtureData{
				Chrome:      f.cr,
				MountPath:   f.mountPath,
				TestAPIConn: f.tconn,
				APIClient:   f.APIClient,
				DriveFs:     f.driveFs,
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

		var err error
		f.cr, err = chrome.New(ctx, append(f.chromeOptions,
			chrome.GAIALoginPool(s.RequiredVar("drivefs.accountPool")),
			chrome.TestExtOAuthClientID(s.RequiredVar("drivefs.extensionClientID")),
			chrome.ARCDisabled())...)
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
	}()

	dfs, err := NewDriveFs(ctx, f.cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed waiting for DriveFS to start: ", err)
	}
	s.Log("drivefs fully started")
	f.driveFs = dfs
	f.mountPath = f.driveFs.MountPath()

	if len(f.drivefsOptions) > 0 {
		var options []string
		for flag, value := range f.drivefsOptions {
			options = append(options, fmt.Sprintf("%s:%s", flag, value))
		}
		cliArgs := fmt.Sprintf("--features=%s", strings.Join(options, ","))
		if err := f.driveFs.WriteCommandLineFlags(cliArgs); err != nil {
			s.Fatal("Failed to write command line args: ", err)
		}
		if err := f.driveFs.Restart(ctx); err != nil {
			s.Fatal("Failed waiting for DriveFS to restart: ", err)
		}
	}

	tconn, err := f.cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed creating test API connection: ", err)
	}
	f.tconn = tconn

	// Perform Drive API authentication.
	ts := NewExtensionTokenSourceForAccount(
		s.FixtContext(),
		f.cr, tconn, driveAPIScopes, f.cr.Creds().User)
	rts := RetryTokenSource(ts, WithContext(s.FixtContext()), WithDelay(time.Second*5))
	apiClient, err := CreateAPIClient(ctx, rts)
	if err != nil {
		s.Fatal("Failed to create Drive API client: ", err)
	}
	f.APIClient = apiClient

	// Lock Chrome and make sure deferred function does not run cleanup.
	chrome.Lock()
	shouldClose = false

	return &FixtureData{
		Chrome:      f.cr,
		MountPath:   f.mountPath,
		TestAPIConn: f.tconn,
		APIClient:   f.APIClient,
		DriveFs:     f.driveFs,
	}
}

// TearDown ensures Chrome is unlocked and closed.
func (f *fixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()
	f.cleanUp(ctx, s)
}

func (f *fixture) Reset(ctx context.Context) error {
	return nil
}

func (f *fixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *fixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

// cleanUp closes Chrome, resets the mountPath to empty string and sets tconn to nil.
func (f *fixture) cleanUp(ctx context.Context, s *testing.FixtState) {
	f.tconn = nil
	f.APIClient = nil

	if len(f.drivefsOptions) > 0 && f.driveFs != nil {
		if err := f.driveFs.ClearCommandLineFlags(); err != nil {
			s.Fatal("Failed to remove command line args file: ", err)
		}
	}
	f.driveFs = nil
	f.mountPath = ""

	if f.cr != nil {
		if err := f.cr.Close(ctx); err != nil {
			s.Log("Failed closing chrome: ", err)
		}
		f.cr = nil
	}
}

// getRefreshTokenForAccount returns the matching refresh token for the
// supplied account. The tokens are stored in a multi line strings as key value
// pairs separated by a ':' character.
func getRefreshTokenForAccount(account, refreshTokens string) (string, error) {
	for i, line := range strings.Split(refreshTokens, "\n") {
		line = strings.TrimSpace(line)
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		ps := strings.SplitN(line, ":", 2)
		if len(ps) != 2 {
			return "", errors.Errorf("failed to parse refresh token list: line %d: does not contain a colon", i+1)
		}
		if ps[0] == account {
			return ps[1], nil
		}
	}
	return "", errors.Errorf("failed to retrieve account token for %q", account)
}
