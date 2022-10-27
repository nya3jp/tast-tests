// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mtp implements the fixture for setting up the connected android device.
package mtp

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	localadb "chromiumos/tast/local/android/adb"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

// resetTimeout is the timeout duration of trying to reset the current fixture.
const resetTimeout = 30 * time.Second

// NewMTPFixture creates a new implementation of MTP fixture with an Android device.
func NewMTPFixture(User, Password string, opts ...chrome.Option) testing.FixtureImpl {
	return &mtpFixture{
		opts:     opts,
		User:     User,
		Password: Password,
	}
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     "mtpWithAndroid",
		Desc:     "User login with ARC enabled and secondary connected Android phone setup in MTP mode",
		Contacts: []string{"cpiao@google.com", "arc-storage@google.com"},
		Impl:     NewMTPFixture("arc.MTP.user", "arc.MTP.password", chrome.ARCEnabled(), chrome.ExtraArgs(arc.DisableSyncFlags()...)),
		Vars: []string{
			"arc.MTP.user",
			"arc.MTP.password",
		},
		SetUpTimeout:    chrome.GAIALoginTimeout + arc.BootTimeout,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
}

type mtpFixture struct {
	cr       *chrome.Chrome
	opts     []chrome.Option
	User     string
	Password string
}

// FixtData holds information made available to tests that specify this Fixture.
type FixtData struct {
	Chrome   *chrome.Chrome
	TestConn *chrome.TestConn
}

func (f *mtpFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	User := s.RequiredVar(f.User)
	Pass := s.RequiredVar(f.Password)

	f.opts = append(f.opts, chrome.GAIALogin(chrome.Creds{
		User: User,
		Pass: Pass,
	}))

	cr, err := chrome.New(ctx, f.opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	f.cr = cr
	fixtData := &FixtData{
		Chrome:   cr,
		TestConn: tconn,
	}

	// Setup adb and connect to the Android phone.
	adbDevice, err := ADBSetUp(ctx)
	if err != nil {
		s.Fatal("Failed to setup an adb device: ", err)
	}

	testing.ContextLog(ctx, "Set to MTP Mode")
	mtpCmd := adbDevice.ShellCommand(ctx, "svc", "usb", "setFunctions", "mtp", "true")
	if _, err := mtpCmd.Output(); err != nil {
		s.Fatal("Failed to set the device to MTP mode: ", err)
	}

	testing.ContextLog(ctx, "Enable adb root")
	rootCmd := adbDevice.Command(ctx, "root")
	// adb requires wait for restart as root for first run after a new build is flashed.
	if err := rootCmd.Run(testexec.DumpLogOnError); err != nil {
		s.Log("Wait until adb restarts as root: ", err)
		_, err := adb.WaitForDevice(ctx, func(device *adb.Device) bool { return !strings.HasPrefix(device.Serial, "emulator-") }, 10*time.Second)
		if err != nil {
			s.Fatal("Failed to restart adb as root: ", err)
		}
	}

	downloadsPath, err := cryptohome.DownloadsPath(ctx, f.cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to retrieve user's Downloads path: ", err)
	}
	// Set up the test file.
	const textFile = "storage.txt"
	testFileLocation := filepath.Join(downloadsPath, textFile)
	if err := ioutil.WriteFile(testFileLocation, []byte("this is a test"), 0777); err != nil {
		s.Fatalf("Creating file %s failed: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

	if err := adbDevice.PushFile(ctx, testFileLocation, "/mnt/sdcard/Download/"); err != nil {
		s.Fatal("Failed to push file to MTP: ", err)
	}

	chrome.Lock()
	return fixtData
}

func (f *mtpFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	f.cr = nil
}

func (f *mtpFixture) Reset(ctx context.Context) error {
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}
	return nil
}

func (f *mtpFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *mtpFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

// ADBSetUp configures adb and connects to the Android device.
func ADBSetUp(ctx context.Context) (*adb.Device, error) {
	// Load the ARC adb vendor key, which must be pre-loaded on the Android device to allow adb over usb without requiring UI interaction.
	if err := localadb.LaunchServer(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to launch adb server")
	}
	// Wait for the first available device, since we are assuming only a single Android device is connected to each CrOS device.
	adbDevice, err := adb.WaitForDevice(ctx, func(device *adb.Device) bool { return !strings.HasPrefix(device.Serial, "emulator-") }, 10*time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list adb devices")
	}
	return adbDevice, nil
}
