// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drivefs

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const (
	driveFsSetupTimeout            = time.Minute
	driveFsCommandLineArgsFilePath = "/home/chronos/user/GCache/v2/%s/command_line_args"
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
			"drivefs.clientCredentials",
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
			"drivefs.clientCredentials",
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
	mountPath      string // The path where Drivefs is mounted
	cr             *chrome.Chrome
	tconn          *chrome.TestConn
	APIClient      *APIClient
	chromeOptions  []chrome.Option
	drivefsOptions map[string]string
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

		var err error
		f.cr, err = chrome.New(ctx, append(f.chromeOptions, chrome.GAIALoginPool(s.RequiredVar("drivefs.accountPool")), chrome.ARCDisabled())...)
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

	if len(f.drivefsOptions) > 0 {
		var options []string
		for flag, value := range f.drivefsOptions {
			options = append(options, fmt.Sprintf("%s:%s", flag, value))
		}
		cliArgs := fmt.Sprintf("--features=%s", strings.Join(options, ","))

		// The command_line_args file must be placed at ~/GCache/v2/[persistableToken].
		persistableToken := getPersistableToken(f.mountPath)
		if len(persistableToken) == 0 {
			s.Fatal("Failed to obtain the drive persistable token: ", f.mountPath)
		}

		if err := ioutil.WriteFile(fmt.Sprintf(driveFsCommandLineArgsFilePath, persistableToken), []byte(cliArgs), 0644); err != nil {
			s.Fatal("Failed to write command line args: ", err)
		}

		// Kill DriveFS, cros-disks will ensure another starts up.
		if err := testexec.CommandContext(ctx, "pkill", "-HUP", "drivefs").Run(); err != nil {
			// pkill exits with code 1 if it could find no matching process (see: man 1 pkill).
			// As it has not started, this is an acceptable as the next start will
			// use the new command line arguments.
			if ws, ok := testexec.GetWaitStatus(err); !ok || !ws.Exited() || ws.ExitStatus() != 1 {
				return errors.Wrap(err, "failed to kill crash_sender processes")
			}
		}

		if _, err := WaitForDriveFs(ctx, f.cr.NormalizedUser()); err != nil {
			s.Fatal("Failed waiting for DriveFS to restart: ", err)
		}
	}

	tconn, err := f.cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed creating test API connection: ", err)
	}
	f.tconn = tconn

	jsonCredentials := s.RequiredVar("drivefs.clientCredentials")
	token, err := getRefreshToken(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get refresh token for account: ", err)
	}

	// Perform Drive API authentication.
	APIClient, err := CreateAPIClient(ctx, f.cr, jsonCredentials, token)
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

	if len(f.drivefsOptions) > 0 {
		persistableToken := getPersistableToken(f.mountPath)
		if len(persistableToken) == 0 {
			s.Fatal("Failed to obtain the drive persistable token from mount path: ", f.mountPath)
		}

		if err := os.Remove(fmt.Sprintf(driveFsCommandLineArgsFilePath, persistableToken)); err != nil {
			s.Fatal("Failed to remove command line args file: ", err)
		}
	}
	f.mountPath = ""

	if f.cr != nil {
		if err := f.cr.Close(ctx); err != nil {
			s.Log("Failed closing chrome: ", err)
		}
		f.cr = nil
	}
}

// getRefreshToken returns an OAuth2 access token for the logged in user's
// primary account, with scope https://www.googleapis.com/auth/drive.
func getRefreshToken(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	var token string
	if err := tconn.Call(ctx, &token, "tast.promisify(chrome.autotestPrivate.getDriveFsToken)"); err != nil {
		return "", errors.Wrap(err, "failed to get access token")
	}
	return token, nil
}

// getPersistableToken derives the token from the mount path. This is used
// to identify the user account directory under ~/GCache/v2.
func getPersistableToken(mountPath string) string {
	return strings.TrimPrefix(mountPath, "/media/fuse/drivefs-")
}
