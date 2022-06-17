// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyfixture

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/common/android/adb"
	nearbycommon "chromiumos/tast/common/cros/nearbyshare"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/crossdevice"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysnippet"
	"chromiumos/tast/local/chrome/nearbyshare/nearbytestutils"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

// resetTimeout is the timeout duration to trying reset of the current fixture.
const resetTimeout = 30 * time.Second

// postTestTimeout is the timeout for the fixture PostTest stage.
// We need a considerable amount of time to collect an Android bug report on failure.
const postTestTimeout = resetTimeout + crossdevice.BugReportDuration

// If skipping the Android login on a 'Some contacts' visibility test, you must specify the logged in Android username as -var=android_username="<username>" so we can configure the CrOS device's allowed Nearby contacts.
const customAndroidUsername = "android_username"

// fixtureOptions holds different Nearby Share settings and configurations for the fixtures.
type fixtureOptions struct {
	crosDataUsage              nearbycommon.DataUsage
	crosVisibility             nearbycommon.Visibility
	androidDataUsage           nearbysnippet.DataUsage
	androidVisibility          nearbysnippet.Visibility
	crosSelectAndroidAsContact bool
}

// NewNearbyShareFixture creates a new implementation of the Nearby Share fixture.
func NewNearbyShareFixture(opts fixtureOptions) testing.FixtureImpl {
	return &nearbyShareFixture{
		crosDataUsage:              opts.crosDataUsage,
		crosVisibility:             opts.crosVisibility,
		crosSelectAndroidAsContact: opts.crosSelectAndroidAsContact,
		androidDataUsage:           opts.androidDataUsage,
		androidVisibility:          opts.androidVisibility,
	}
}

func init() {
	// We have a lot of fixtures for all the various Nearby Share use cases we test.
	// Add new non-parent fixtures in separate files and functions to keep this one manageable.
	addModulefoodAndroidFixtures()
	addProdAndroidFixtures()
	addDevAndroidFixtures()
	addBackgroundScanningFixtures()
	addARCFixtures()
	addWebRTCAndWLANFixtures()
}

type nearbyShareFixture struct {
	cr             *chrome.Chrome
	crosDataUsage  nearbycommon.DataUsage
	crosVisibility nearbycommon.Visibility
	// crosSelectAndroidAsContact is only used when crosVisibility == nearbycommon.VisibilitySelectedContacts. If true, the connected Android device will be selected as an allowed contact. Otherwise no contacts will be selected.
	crosSelectAndroidAsContact bool
	androidDataUsage           nearbysnippet.DataUsage
	androidVisibility          nearbysnippet.Visibility
	androidDevice              *nearbysnippet.AndroidNearbyDevice
	crosAttributes             *nearbycommon.CrosAttributes
	androidAttributes          *nearbysnippet.AndroidAttributes
	// ChromeReader is the line reader for collecting Chrome logs.
	ChromeReader *syslog.LineReader
	// createBtsnoopCmd returns the command for btsnoop log capture. The command is started in PreTest and must be killed in PostTest before saving the logs.
	createBtsnoopCmd func(string) *testexec.Cmd
	btsnoopCmd       *testexec.Cmd
	logcatStartTime  adb.LogcatTimestamp
}

// FixtData holds information made available to tests that specify this Fixture.
type FixtData struct {
	// Chrome is the running chrome instance.
	Chrome *chrome.Chrome

	// TestConn is a connection to the test extension.
	TestConn *chrome.TestConn

	// CrOSUsername is the user account logged into Chrome.
	CrOSUsername string

	// CrOSDeviceName is the CrOS device name configured for Nearby Share.
	CrOSDeviceName string

	// AndroidDevice is an object for interacting with the connected Android device's Snippet Library.
	AndroidDevice *nearbysnippet.AndroidNearbyDevice

	// AndroidDeviceName is the Android device name configured for Nearby Share.
	AndroidDeviceName string

	// AndroidLoggedIn is true if Android is logged in.
	AndroidLoggedIn bool

	// AndroidUsername is the GAIA account logged in on Android.
	AndroidUsername string

	// AndroidNearbyChannel is the channel of Nearby the phone is using.
	AndroidNearbyChannel channel

	// ARC is the ARC instance, if enabled.
	ARC *arc.ARC
}

func (f *nearbyShareFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	cr := s.ParentValue().(*FixtData).Chrome
	crosUsername := s.ParentValue().(*FixtData).CrOSUsername
	androidDevice := s.ParentValue().(*FixtData).AndroidDevice
	androidDeviceName := s.ParentValue().(*FixtData).AndroidDeviceName
	androidUsername := s.ParentValue().(*FixtData).AndroidUsername
	loggedIn := s.ParentValue().(*FixtData).AndroidLoggedIn

	// Allocate time for logging and taking a bugreport in case of failure.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, crossdevice.BugReportDuration)
	defer cancel()

	// Save logcat so we have Android logs even if fixture setup fails.
	startTime, err := androidDevice.Device.LatestLogcatTimestamp(ctx)
	if err != nil {
		s.Fatal("Failed to get latest logcat timestamp: ", err)
	}
	defer androidDevice.Device.DumpLogcatFromTimestamp(cleanupCtx, filepath.Join(s.OutDir(), "fixture_setup_logcat.txt"), startTime)
	defer androidDevice.DumpLogs(cleanupCtx, s.OutDir(), "fixture_setup_persistent_logcat.txt")

	// Save an Android bug report on setup failure.
	defer func() {
		if s.HasError() {
			if err := crossdevice.BugReport(ctx, androidDevice.Device, s.OutDir()); err != nil {
				s.Log("Failed to save Android bug report: ", err)
			}
		}
	}()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Set up Nearby Share on the CrOS device.
	const crosBaseName = "cros_test"
	crosDisplayName := nearbycommon.RandomDeviceName(crosBaseName)
	if err := nearbyshare.CrOSSetup(ctx, tconn, cr, f.crosDataUsage, f.crosVisibility, crosDisplayName); err != nil {
		s.Fatal("Failed to set up Nearby Share: ", err)
	}

	// Check if Android settings need to be updated. Keep name the same though.
	v, err := androidDevice.GetVisibility(ctx)
	if err != nil {
		s.Fatal("Failed to read Android Visibilty setting: ", err)
	}
	d, err := androidDevice.GetDataUsage(ctx)
	if err != nil {
		s.Fatal("Failed to read Android Data Usage setting: ", err)
	}
	if f.androidDataUsage != d || f.androidVisibility != v {
		if err := configureAndroidNearbySettings(ctx, androidDevice, f.androidDataUsage, f.androidVisibility, androidDeviceName); err != nil {
			s.Fatal("Failed to configure Android Nearby Share settings: ", err)
		}
	}

	// Store CrOS test metadata for reporting.
	crosAttributes, err := nearbyshare.GetCrosAttributes(ctx, tconn, crosDisplayName, crosUsername, f.crosDataUsage, f.crosVisibility)
	if err != nil {
		s.Fatal("Failed to get CrOS attributes for reporting: ", err)
	}
	f.crosAttributes = crosAttributes

	f.cr = cr
	f.androidDevice = androidDevice
	fixData := &FixtData{
		Chrome:            cr,
		TestConn:          tconn,
		CrOSDeviceName:    crosDisplayName,
		AndroidDevice:     androidDevice,
		AndroidDeviceName: androidDeviceName,
		ARC:               s.ParentValue().(*FixtData).ARC,
	}

	// Store Android attributes for reporting.
	androidAttributes, err := androidDevice.GetAndroidAttributes(ctx)
	if err != nil {
		s.Fatal("Failed to get Android attributes for reporting: ", err)
	}
	f.androidAttributes = androidAttributes

	// Set the Android device as an allowed contact if the CrOS visibility setting is 'Some contacts'.
	if f.crosVisibility == nearbycommon.VisibilitySelectedContacts && f.crosSelectAndroidAsContact {
		nearbySettings, err := nearbyshare.LaunchNearbySettings(ctx, tconn, cr)
		if err != nil {
			s.Fatal("Failed to launch OS settings: ", err)
		}
		defer nearbySettings.Close(ctx)

		androidContact := androidUsername
		if loggedIn {
			if val, ok := s.Var("android_username"); ok {
				androidContact = val
			} else {
				s.Fatal("android_username var must be provided if skipping Android login for a 'Some contacts' visibility test. Please provide the username of the connected Android device")
			}
		}

		if err := nearbySettings.SetAllowedContacts(ctx, androidContact); err != nil {
			s.Fatal("Failed to set allowed contacts: ", err)
		}
	}

	// TODO(crbug/1189962): To save the btsnoop logs for the duration of each test, we need to start this command in PreTest and kill it in PostTest.
	// The only way to do that at the moment is to initialize it with the fixture's context, since PreTest's context is cancelled when it returns and the command won't run.
	// Move creating the command to PreTest once test-scoped context is accessible within PreTest.
	f.createBtsnoopCmd = func(outDir string) *testexec.Cmd {
		return bluetooth.StartBTSnoopLogging(s.FixtContext(), filepath.Join(outDir, nearbycommon.BtsnoopLog))
	}

	return fixData
}

func (f *nearbyShareFixture) TearDown(ctx context.Context, s *testing.FixtState) {}

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
	chromeReader, err := nearbyshare.StartLogging(ctx, syslog.ChromeLogFile)
	if err != nil {
		s.Error("Failed to start Chrome logging: ", err)
	}
	f.ChromeReader = chromeReader

	timestamp, err := f.androidDevice.Device.LatestLogcatTimestamp(ctx)
	if err != nil {
		s.Fatal("Failed to get latest logcat timestamp: ", err)
	}
	f.logcatStartTime = timestamp

	if err := saveDeviceAttributes(f.crosAttributes, f.androidAttributes, filepath.Join(s.OutDir(), "device_attributes.json")); err != nil {
		s.Error("Failed to save device attributes: ", err)
	}
	f.btsnoopCmd = f.createBtsnoopCmd(s.OutDir())
	if err := f.btsnoopCmd.Start(); err != nil {
		s.Fatal("Failed to start btsnoop log: ", err)
	}
}

func (f *nearbyShareFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// Save CrOS and Android device logs.
	if f.ChromeReader == nil {
		s.Error("ChromeReader not defined")
	}
	if err := nearbyshare.SaveLogs(ctx, f.ChromeReader, filepath.Join(s.OutDir(), nearbycommon.ChromeLog)); err != nil {
		s.Error("Failed to save Chrome log: ", err)
	}

	if err := f.androidDevice.Device.DumpLogcatFromTimestamp(ctx, filepath.Join(s.OutDir(), "nearby-logcat.txt"), f.logcatStartTime); err != nil {
		s.Fatal("Failed to save logcat logs from the test: ", err)
	}
	if err := f.androidDevice.DumpLogs(ctx, s.OutDir(), "nearby-persistent-logcat.txt"); err != nil {
		s.Fatal("Failed to save persistent logcat logs: ", err)
	}

	if err := f.btsnoopCmd.Kill(); err != nil {
		s.Error("Failed to stop btsnoop log capture: ", err)
	}
	f.btsnoopCmd = nil

	// Clear test files from both devices.
	if err := nearbytestutils.ClearCrOSDownloads(ctx); err != nil {
		s.Error("Failed to clear contents of the CrOS downloads folder: ", err)
	}
	if err := f.androidDevice.ClearDownloads(ctx); err != nil {
		s.Error("Failed to clear contents of the Android downloads folder: ", err)
	}

	if s.HasError() {
		if err := crossdevice.BugReport(ctx, f.androidDevice.Device, s.OutDir()); err != nil {
			s.Error("Failed to save Android bug report: ", err)
		}
	}
}

// saveDeviceAttributes saves the CrOS and Android device attributes as a formatted JSON at the specified filepath.
func saveDeviceAttributes(crosAttrs *nearbycommon.CrosAttributes, androidAttrs *nearbysnippet.AndroidAttributes, filepath string) error {
	attributes := struct {
		CrOS    *nearbycommon.CrosAttributes
		Android *nearbysnippet.AndroidAttributes
	}{CrOS: crosAttrs, Android: androidAttrs}
	crosLog, err := json.MarshalIndent(attributes, "", "\t")
	if err != nil {
		return errors.Wrap(err, "failed to format device metadata for logging")
	}
	if err := ioutil.WriteFile(filepath, crosLog, 0644); err != nil {
		return errors.Wrap(err, "failed to write CrOS attributes to output file")
	}
	return nil
}
