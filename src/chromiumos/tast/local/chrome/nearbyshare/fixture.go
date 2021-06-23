// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"time"

	nearbycommon "chromiumos/tast/common/cros/nearbyshare"
	"chromiumos/tast/common/cros/nearbyshare/nearbysetup"
	"chromiumos/tast/common/cros/nearbyshare/nearbytestutils"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysnippet"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

// resetTimeout is the timeout duration to trying reset of the current fixture.
const resetTimeout = 30 * time.Second

// NewNearbyShareFixture creates a new implementation of the Nearby Share fixture.
func NewNearbyShareFixture(crosDataUsage nearbysetup.DataUsage, crosVisibility nearbysetup.Visibility, androidDataUsage nearbysnippet.DataUsage, androidVisibility nearbysnippet.Visibility, crosSelectAndroidAsContact bool) testing.FixtureImpl {
	return &nearbyShareFixture{
		crosDataUsage:              crosDataUsage,
		crosVisibility:             crosVisibility,
		crosSelectAndroidAsContact: crosSelectAndroidAsContact,
		androidDataUsage:           androidDataUsage,
		androidVisibility:          androidVisibility,
	}
}

func init() {
	const (
		// If skipping the Android login on a 'Some contacts' visibility test, you must specify the logged in Android username as -var=android_username="<username>" so we can configure the CrOS device's allowed Nearby contacts.
		customAndroidUsername = "android_username"
	)

	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareDataUsageOfflineAllContacts",
		Desc: "Nearby Share enabled on CrOS and Android configured with 'Data Usage' set to 'Offline',  'Visibility' set to 'All Contacts'",
		Impl: NewNearbyShareFixture(nearbysetup.DataUsageOffline, nearbysetup.VisibilityAllContacts, nearbysnippet.DataUsageOffline, nearbysnippet.VisibilityAllContacts, false),
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Parent:          "nearbyShareGAIALogin",
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:   "nearbyShareDataUsageOnlineAllContacts",
		Desc:   "Nearby Share enabled on CrOS and Android configured with 'Data Usage' set to 'Online',  'Visibility' set to 'All Contacts'",
		Parent: "nearbyShareGAIALogin",
		Impl:   NewNearbyShareFixture(nearbysetup.DataUsageOnline, nearbysetup.VisibilityAllContacts, nearbysnippet.DataUsageOnline, nearbysnippet.VisibilityAllContacts, false),
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:   "nearbyShareDataUsageOnlineNoOne",
		Desc:   "Nearby Share enabled on CrOS and Android configured with 'Data Usage' set to 'Online' and 'Visibility' set to 'No One'",
		Parent: "nearbyShareGAIALogin",
		Impl:   NewNearbyShareFixture(nearbysetup.DataUsageOnline, nearbysetup.VisibilityNoOne, nearbysnippet.DataUsageOnline, nearbysnippet.VisibilityNoOne, false),
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareDataUsageOfflineNoOne",
		Desc: "Nearby Share enabled on CrOS and Android configured with 'Data Usage' set to 'Offline' and 'Visibility' set to 'No One'",
		Impl: NewNearbyShareFixture(nearbysetup.DataUsageOffline, nearbysetup.VisibilityNoOne, nearbysnippet.DataUsageOffline, nearbysnippet.VisibilityNoOne, false),
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Parent:          "nearbyShareGAIALogin",
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareDataUsageOfflineSomeContactsAndroidSelectedContact",
		Desc: "Nearby Share enabled on CrOS and Android with 'Data Usage' set to 'Offline' on both. The Android device 'Visibility' is 'All Contacts'. The CrOS device 'Visibility' is 'Some contacts' with the Android user set as an allowed contact so it will be visible to the Android device. The CrOS device is logged in with a GAIA account which is mutual contacts with the Android GAIA account",
		Impl: NewNearbyShareFixture(nearbysetup.DataUsageOffline, nearbysetup.VisibilitySelectedContacts, nearbysnippet.DataUsageOffline, nearbysnippet.VisibilityAllContacts, true),
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Parent: "nearbyShareGAIALogin",
		Vars: []string{
			customAndroidUsername,
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:   "nearbyShareDataUsageOnlineSomeContactsAndroidSelectedContact",
		Desc:   "Nearby Share enabled on CrOS and Android with 'Data Usage' set to 'Online' on both. The Android device 'Visibility' is 'All Contacts'. The CrOS device 'Visibility' is 'Some contacts' with the Android user set as an allowed contact so it will be visible to the Android device. The CrOS device is logged in with a GAIA account which is mutual contacts with the Android GAIA account",
		Parent: "nearbyShareGAIALogin",
		Impl:   NewNearbyShareFixture(nearbysetup.DataUsageOnline, nearbysetup.VisibilitySelectedContacts, nearbysnippet.DataUsageOnline, nearbysnippet.VisibilityAllContacts, true),
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Vars: []string{
			customAndroidUsername,
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name:   "nearbyShareDataUsageOfflineSomeContactsAndroidNotSelectedContact",
		Desc:   "Nearby Share enabled on CrOS and Android with 'Data Usage' set to 'Offline' on both. The Android device 'Visibility' is 'All Contacts'. The CrOS device 'Visibility' is 'Some contacts' with no contacts selected, so it will not be visible to the Android device. The CrOS device is logged in with a GAIA account which is mutual contacts with the Android GAIA account",
		Parent: "nearbyShareGAIALogin",
		Impl:   NewNearbyShareFixture(nearbysetup.DataUsageOffline, nearbysetup.VisibilitySelectedContacts, nearbysnippet.DataUsageOffline, nearbysnippet.VisibilityAllContacts, false),
		Vars: []string{
			customAndroidUsername,
		},
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
}

type nearbyShareFixture struct {
	cr             *chrome.Chrome
	crosDataUsage  nearbysetup.DataUsage
	crosVisibility nearbysetup.Visibility
	// crosSelectAndroidAsContact is only used when crosVisibility == nearbysetup.VisibilitySelectedContacts. If true, the connected Android device will be selected as an allowed contact. Otherwise no contacts will be selected.
	crosSelectAndroidAsContact bool
	androidDataUsage           nearbysnippet.DataUsage
	androidVisibility          nearbysnippet.Visibility
	androidDevice              *nearbysnippet.AndroidNearbyDevice
	crosAttributes             *nearbysetup.CrosAttributes
	androidAttributes          *nearbysnippet.AndroidAttributes
	// ChromeReader is the line reader for collecting Chrome logs.
	ChromeReader *syslog.LineReader
	// createBtsnoopCmd returns the command for btsnoop log capture. The command is started in PreTest and must be killed in PostTest before saving the logs.
	createBtsnoopCmd func(string) *testexec.Cmd
	btsnoopCmd       *testexec.Cmd
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
}

func (f *nearbyShareFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	cr := s.ParentValue().(*FixtData).Chrome
	crosUsername := s.ParentValue().(*FixtData).CrOSUsername
	androidDevice := s.ParentValue().(*FixtData).AndroidDevice
	androidDeviceName := s.ParentValue().(*FixtData).AndroidDeviceName
	androidUsername := s.ParentValue().(*FixtData).AndroidUsername
	loggedIn := s.ParentValue().(*FixtData).AndroidLoggedIn

	// Reset and save logcat so we have Android logs even if fixture setup fails.
	if err := androidDevice.ClearLogcat(ctx); err != nil {
		s.Fatal("Failed to clear logcat at start of fixture setup")
	}
	defer androidDevice.DumpLogs(ctx, s.OutDir(), "fixture_setup_logcat.txt")

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Set up Nearby Share on the CrOS device.
	const crosBaseName = "cros_test"
	crosDisplayName := nearbytestutils.RandomDeviceName(crosBaseName)
	if err := nearbysetup.CrOSSetup(ctx, tconn, cr, f.crosDataUsage, f.crosVisibility, crosDisplayName); err != nil {
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
		if err := nearbysetup.AndroidConfigure(ctx, androidDevice, f.androidDataUsage, f.androidVisibility, androidDeviceName); err != nil {
			s.Fatal("Failed to configure Android Nearby Share settings: ", err)
		}
	}

	// Store CrOS test metadata for reporting.
	crosAttributes, err := nearbysetup.GetCrosAttributes(ctx, tconn, crosDisplayName, crosUsername, f.crosDataUsage, f.crosVisibility)
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
	}

	// Store Android attributes for reporting.
	androidAttributes, err := androidDevice.GetAndroidAttributes(ctx)
	if err != nil {
		s.Fatal("Failed to get Android attributes for reporting: ", err)
	}
	f.androidAttributes = androidAttributes

	// Set the Android device as an allowed contact if the CrOS visibility setting is 'Some contacts'.
	if f.crosVisibility == nearbysetup.VisibilitySelectedContacts && f.crosSelectAndroidAsContact {
		nearbySettings, err := LaunchNearbySettings(ctx, tconn, cr)
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
	chromeReader, err := nearbytestutils.StartLogging(ctx, syslog.ChromeLogFile)
	if err != nil {
		s.Error("Failed to start Chrome logging: ", err)
	}
	f.ChromeReader = chromeReader
	if err := f.androidDevice.ClearLogcat(ctx); err != nil {
		s.Fatal("Failed to clear logcat before the test run: ", err)
	}
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
	if err := nearbytestutils.SaveLogs(ctx, f.ChromeReader, filepath.Join(s.OutDir(), nearbycommon.ChromeLog)); err != nil {
		s.Error("Failed to save Chrome log: ", err)
	}
	if err := f.androidDevice.DumpLogs(ctx, s.OutDir(), "nearby_logcat.txt"); err != nil {
		s.Error("Failed to save Android logcat: ", err)
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
}

// saveDeviceAttributes saves the CrOS and Android device attributes as a formatted JSON at the specified filepath.
func saveDeviceAttributes(crosAttrs *nearbysetup.CrosAttributes, androidAttrs *nearbysnippet.AndroidAttributes, filepath string) error {
	attributes := struct {
		CrOS    *nearbysetup.CrosAttributes
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
