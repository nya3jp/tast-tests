// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	nearbycommon "chromiumos/tast/common/cros/nearbyshare"
	"chromiumos/tast/common/cros/nearbyshare/nearbysetup"
	"chromiumos/tast/common/cros/nearbyshare/nearbytestutils"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysnippet"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

// resetTimeout is the timeout duration to trying reset of the current fixture.
const resetTimeout = 30 * time.Second

// NewNearbyShareFixtureWithAndroid creates a new implementation of the Nearby Share fixture with an Android device.
func NewNearbyShareFixtureWithAndroid(crosDataUsage nearbysetup.DataUsage, crosVisibility nearbysetup.Visibility, androidDataUsage nearbysnippet.DataUsage, androidVisibility nearbysnippet.Visibility, gaiaLogin, crosSelectAndroidAsContact bool, opts ...chrome.Option) testing.FixtureImpl {
	defaultNearbyOpts := []chrome.Option{
		chrome.EnableFeatures("IntentHandlingSharing", "NearbySharing", "Sharesheet"),
		chrome.ExtraArgs("--nearby-share-verbose-logging"),
	}
	return &nearbyShareFixture{
		opts:                       append(defaultNearbyOpts, opts...),
		crosDataUsage:              crosDataUsage,
		crosVisibility:             crosVisibility,
		crosSelectAndroidAsContact: crosSelectAndroidAsContact,
		androidDataUsage:           androidDataUsage,
		androidVisibility:          androidVisibility,
		gaiaLogin:                  gaiaLogin,
		androidSetup:               true,
	}
}

func init() {
	const (
		// These are the default GAIA credentials that will be used to sign in on the devices. Use the optional "custom" vars below to specify you'd like to specify your own credentials while running locally on personal devices.
		defaultCrOSUsername    = "nearbyshare.cros_username"
		defaultCrOSPassword    = "nearbyshare.cros_password"
		defaultAndroidUsername = "nearbyshare.android_username"
		defaultAndroidPassword = "nearbyshare.android_password"

		// These vars can be used from the command line when running tests locally to configure the tests to run on non-rooted devices and personal GAIA accounts.
		// Specify -var=rooted=false when running on an unrooted device to skip steps that require adb root access.
		rooted = "rooted"
		// Specify -var=skipAndroidLogin=true if the Android device is logged in to a personal account. Otherwise we will attempt removing all Google accounts and adding a test account to the phone.
		skipAndroidLogin = "skipAndroidLogin"
		// If skipping the Android login on a 'Some contacts' visibility test, you must specify the logged in Android username as -var=android_username="<username>" so we can configure the CrOS device's allowed Nearby contacts.
		customAndroidUsername = "android_username"
		// Use these vars to log in with your own GAIA credentials. If running in-contacts tests with an Android device, it is expected that the CrOS user and Android user are already mutual contacts.
		customCrOSUsername = "cros_username"
		customCrOSPassword = "cros_password"
	)

	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareDataUsageOfflineAllContactsTestUser",
		Desc: "Nearby Share enabled on CrOS and Android configured with 'Data Usage' set to 'Offline' and 'Visibility' set to 'All Contacts'",
		Impl: NewNearbyShareFixtureWithAndroid(nearbysetup.DataUsageOffline, nearbysetup.VisibilityAllContacts, nearbysnippet.DataUsageOffline, nearbysnippet.VisibilityAllContacts, false, false),
		Vars: []string{
			defaultAndroidUsername,
			defaultAndroidPassword,
			rooted,
			skipAndroidLogin,
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareDataUsageOfflineAllContactsGAIA",
		Desc: "Nearby Share enabled on CrOS and Android configured with 'Data Usage' set to 'Offline',  'Visibility' set to 'All Contacts' and logged in with a real GAIA account",
		Impl: NewNearbyShareFixtureWithAndroid(nearbysetup.DataUsageOffline, nearbysetup.VisibilityAllContacts, nearbysnippet.DataUsageOffline, nearbysnippet.VisibilityAllContacts, true, false),
		Vars: []string{
			defaultCrOSUsername,
			defaultCrOSPassword,
			defaultAndroidUsername,
			defaultAndroidPassword,
			rooted,
			skipAndroidLogin,
			customCrOSUsername,
			customCrOSPassword,
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	// Online data usage requires a real GAIA login.
	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareDataUsageOnlineAllContactsGAIA",
		Desc: "Nearby Share enabled on CrOS and Android configured with 'Data Usage' set to 'Online',  'Visibility' set to 'All Contacts' and logged in with a real GAIA account",
		Impl: NewNearbyShareFixtureWithAndroid(nearbysetup.DataUsageOnline, nearbysetup.VisibilityAllContacts, nearbysnippet.DataUsageOnline, nearbysnippet.VisibilityAllContacts, true, false),
		Vars: []string{
			defaultCrOSUsername,
			defaultCrOSPassword,
			defaultAndroidUsername,
			defaultAndroidPassword,
			rooted,
			skipAndroidLogin,
			customCrOSUsername,
			customCrOSPassword,
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareDataUsageOnlineNoOneGAIA",
		Desc: "Nearby Share enabled on CrOS and Android configured with 'Data Usage' set to 'Online' and 'Visibility' set to 'No One' and logged in with a real GAIA account",
		Impl: NewNearbyShareFixtureWithAndroid(nearbysetup.DataUsageOnline, nearbysetup.VisibilityNoOne, nearbysnippet.DataUsageOnline, nearbysnippet.VisibilityNoOne, true, false),
		Vars: []string{
			defaultCrOSUsername,
			defaultCrOSPassword,
			defaultAndroidUsername,
			defaultAndroidPassword,
			rooted,
			skipAndroidLogin,
			customCrOSUsername,
			customCrOSPassword,
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareDataUsageOfflineSomeContactsAndroidSelectedContactGAIA",
		Desc: "Nearby Share enabled on CrOS and Android with 'Data Usage' set to 'Offline' on both. The Android device 'Visibility' is 'All Contacts'. The CrOS device 'Visibility' is 'Some contacts' with the Android user set as an allowed contact so it will be visible to the Android device. The CrOS device is logged in with a GAIA account which is mutual contacts with the Android GAIA account.",
		Impl: NewNearbyShareFixtureWithAndroid(nearbysetup.DataUsageOffline, nearbysetup.VisibilitySelectedContacts, nearbysnippet.DataUsageOffline, nearbysnippet.VisibilityAllContacts, true, true),
		Vars: []string{
			defaultCrOSUsername,
			defaultCrOSPassword,
			defaultAndroidUsername,
			defaultAndroidPassword,
			rooted,
			skipAndroidLogin,
			customAndroidUsername,
			customCrOSUsername,
			customCrOSPassword,
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareDataUsageOnlineSomeContactsAndroidSelectedContactGAIA",
		Desc: "Nearby Share enabled on CrOS and Android with 'Data Usage' set to 'Online' on both. The Android device 'Visibility' is 'All Contacts'. The CrOS device 'Visibility' is 'Some contacts' with the Android user set as an allowed contact so it will be visible to the Android device. The CrOS device is logged in with a GAIA account which is mutual contacts with the Android GAIA account.",
		Impl: NewNearbyShareFixtureWithAndroid(nearbysetup.DataUsageOnline, nearbysetup.VisibilitySelectedContacts, nearbysnippet.DataUsageOnline, nearbysnippet.VisibilityAllContacts, true, true),
		Vars: []string{
			defaultCrOSUsername,
			defaultCrOSPassword,
			defaultAndroidUsername,
			defaultAndroidPassword,
			rooted,
			skipAndroidLogin,
			customAndroidUsername,
			customCrOSUsername,
			customCrOSPassword,
		},
		SetUpTimeout:    2 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})

	testing.AddFixture(&testing.Fixture{
		Name: "nearbyShareDataUsageOfflineSomeContactsAndroidNotSelectedContactGAIA",
		Desc: "Nearby Share enabled on CrOS and Android with 'Data Usage' set to 'Offline' on both. The Android device 'Visibility' is 'All Contacts'. The CrOS device 'Visibility' is 'Some contacts' with no contacts selected, so it will not be visible to the Android device. The CrOS device is logged in with a GAIA account which is mutual contacts with the Android GAIA account.",
		Impl: NewNearbyShareFixtureWithAndroid(nearbysetup.DataUsageOffline, nearbysetup.VisibilitySelectedContacts, nearbysnippet.DataUsageOffline, nearbysnippet.VisibilityAllContacts, true, false),
		Vars: []string{
			defaultCrOSUsername,
			defaultCrOSPassword,
			defaultAndroidUsername,
			defaultAndroidPassword,
			rooted,
			skipAndroidLogin,
			customAndroidUsername,
			customCrOSUsername,
			customCrOSPassword,
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
	opts           []chrome.Option
	crosDataUsage  nearbysetup.DataUsage
	crosVisibility nearbysetup.Visibility
	// crosSelectAndroidAsContact is only used when crosVisibility == nearbysetup.VisibilitySelectedContacts. If true, the connected Android device will be selected as an allowed contact. Otherwise no contacts will be selected.
	crosSelectAndroidAsContact bool
	androidDataUsage           nearbysnippet.DataUsage
	androidVisibility          nearbysnippet.Visibility
	gaiaLogin                  bool
	androidSetup               bool
	androidDevice              *nearbysnippet.AndroidNearbyDevice
	crosAttributes             *nearbysetup.CrosAttributes
	androidAttributes          *nearbysnippet.AndroidAttributes
	// ChromeReader is the line reader for collecting Chrome logs.
	ChromeReader *syslog.LineReader
	// btsnoopCmd is the command for btsnoop log capture that is started in PreTest and must be killed in PostTest before saving the logs.
	btsnoopCmd        *testexec.Cmd
	btsnoopTmpLogPath string
}

// FixtData holds information made available to tests that specify this Fixture.
type FixtData struct {
	// Chrome is the running chrome instance.
	Chrome *chrome.Chrome

	// TestConn is a connection to the test extension.
	TestConn *chrome.TestConn

	// CrOSDeviceName is the CrOS device name configured for Nearby Share.
	CrOSDeviceName string

	// AndroidDevice is an object for interacting with the connected Android device's Snippet Library.
	AndroidDevice *nearbysnippet.AndroidNearbyDevice

	// AndroidDeviceName is the Android device name configured for Nearby Share.
	AndroidDeviceName string
}

func (f *nearbyShareFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	crosUsername := chrome.DefaultUser
	if f.gaiaLogin {
		crosUsername = s.RequiredVar("nearbyshare.cros_username")
		crosPassword := s.RequiredVar("nearbyshare.cros_password")
		customUser, userOk := s.Var("cros_username")
		customPass, passOk := s.Var("cros_password")
		if userOk && passOk {
			s.Log("Logging in with user-provided credentials")
			crosUsername = customUser
			crosPassword = customPass
		} else {
			s.Log("Logging in with default GAIA credentials")
		}
		f.opts = append(f.opts, chrome.GAIALogin(chrome.Creds{User: crosUsername, Pass: crosPassword}))
	}

	cr, err := chrome.New(
		ctx,
		f.opts...,
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

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

	// Store CrOS test metadata for reporting.
	crosAttributes, err := nearbysetup.GetCrosAttributes(ctx, tconn, crosDisplayName, crosUsername, f.crosDataUsage, f.crosVisibility)
	if err != nil {
		s.Fatal("Failed to get CrOS attributes for reporting: ", err)
	}
	f.crosAttributes = crosAttributes

	f.cr = cr
	fixData := &FixtData{
		Chrome:         cr,
		TestConn:       tconn,
		CrOSDeviceName: crosDisplayName,
	}

	if f.androidSetup {
		// Set up Nearby Share on the Android device. Don't override GMS Core flags or perform settings changes that require root access if specified in the runtime vars.
		rooted := true
		if val, ok := s.Var("rooted"); ok {
			b, err := strconv.ParseBool(val)
			if err != nil {
				s.Fatal("Unable to convert rooted var to bool: ", err)
			}
			rooted = b
		}
		const androidBaseName = "android_test"
		androidDisplayName := nearbytestutils.RandomDeviceName(androidBaseName)
		// TODO(crbug/1127165): Replace with s.DataPath(nearbysnippet.ZipName) when data is supported in Fixtures.
		// The data path changes based on whether -build=true or -build=false is supplied to `tast run`.
		// Local test runs on your workstation use -build=true by default, while lab runs use -build=false.
		const (
			prebuiltLocalDataPath = "/usr/local/share/tast/data/chromiumos/tast/local/bundles/cros/nearbyshare/data"
			builtLocalDataPath    = "/usr/local/share/tast/data_pushed/chromiumos/tast/local/bundles/cros/nearbyshare/data"
			apkZipName            = "nearby_snippet.zip"
			accountUtilZipName    = "google_account_util.zip"
		)

		// Use the built local data path if it exists, and fall back to the prebuilt data path otherwise.
		apkZipPath := filepath.Join(builtLocalDataPath, apkZipName)
		accountUtilZipPath := filepath.Join(builtLocalDataPath, accountUtilZipName)
		if _, err := os.Stat(builtLocalDataPath); os.IsNotExist(err) {
			apkZipPath = filepath.Join(prebuiltLocalDataPath, apkZipName)
			accountUtilZipPath = filepath.Join(prebuiltLocalDataPath, accountUtilZipName)
		} else if err != nil {
			s.Fatal("Failed to check if built local data path exists: ", err)
		}
		// Setup adb and connect to the Android phone.
		adbDevice, err := nearbysetup.AdbSetup(ctx)
		if err != nil {
			s.Fatal("Failed to setup an adb device: ", err)
		}
		// We want to ensure we have logs even if the Android device setup fails.
		fixtureLogcatPath := filepath.Join(s.OutDir(), "fixture_setup_logcat.txt")
		defer adbDevice.DumpLogcat(ctx, fixtureLogcatPath)

		// Skip logging in to the test account on the Android device if specified in the runtime vars.
		// This lets you run the tests on a phone that's already signed in with your own account.
		loggedIn := false
		if val, ok := s.Var("skipAndroidLogin"); ok {
			b, err := strconv.ParseBool(val)
			if err != nil {
				s.Fatal("Unable to convert skipAndroidLogin var to bool: ", err)
			}
			loggedIn = b
		}
		androidUsername := s.RequiredVar("nearbyshare.android_username")
		androidPassword := s.RequiredVar("nearbyshare.android_password")

		// Configure the Android phone and setup the Snippet library.
		androidDevice, err := nearbysetup.AndroidSetup(
			ctx, adbDevice, accountUtilZipPath, androidUsername, androidPassword, loggedIn, apkZipPath, rooted,
			nearbysetup.DefaultScreenTimeout,
			f.androidDataUsage,
			f.androidVisibility,
			androidDisplayName,
		)
		if err != nil {
			s.Fatal("Failed to prepare connected Android device for Nearby Share testing: ", err)
		}
		f.androidDevice = androidDevice
		fixData.AndroidDevice = androidDevice
		fixData.AndroidDeviceName = androidDisplayName

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
	}

	// TODO(crbug/1189962): To save the btsnoop logs for the duration of each test, we need to start this command in PreTest and kill it in PostTest.
	// The only way to do that at the moment is to initialize it with the fixture's context, since PreTest's context is cancelled when it returns and the command won't run.
	// Move starting the command to PreTest once test-scoped context is accessible within PreTest.
	tmpLogPath := filepath.Join(os.TempDir(), nearbycommon.BtsnoopLog)
	cmd := bluetooth.StartBTSnoopLogging(s.FixtContext(), tmpLogPath)
	f.btsnoopCmd = cmd
	f.btsnoopTmpLogPath = tmpLogPath

	// Lock chrome after all Setup is complete so we don't block other fixtures.
	chrome.Lock()
	return fixData
}

func (f *nearbyShareFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	chrome.Unlock()
	if err := f.cr.Close(ctx); err != nil {
		s.Log("Failed to close Chrome connection: ", err)
	}
	f.cr = nil
	if f.androidDevice != nil {
		f.androidDevice.StopSnippet(ctx)
	}
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
		s.Error("Failed to start Chrome logging: ", err)
	}
	f.ChromeReader = chromeReader
	if f.androidSetup {
		if err := f.androidDevice.ClearLogcat(ctx); err != nil {
			s.Fatal("Failed to clear logcat before the test run: ", err)
		}
	}
	if err := saveDeviceAttributes(f.crosAttributes, f.androidAttributes, filepath.Join(s.OutDir(), "device_attributes.json")); err != nil {
		s.Error("Failed to save device attributes: ", err)
	}
	if err := f.btsnoopCmd.Start(); err != nil {
		s.Fatal("Failed to start btsnoop log: ", err)
	}
}

func (f *nearbyShareFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if f.ChromeReader == nil {
		s.Error("ChromeReader not defined")
	}
	if err := nearbytestutils.SaveLogs(ctx, f.ChromeReader, filepath.Join(s.OutDir(), nearbycommon.ChromeLog)); err != nil {
		s.Error("Failed to save Chrome log: ", err)
	}
	if f.androidSetup {
		f.androidDevice.DumpLogs(ctx, s.OutDir(), "nearby_logcat.txt")
	}
	if err := f.btsnoopCmd.Kill(); err != nil {
		s.Fatal("Failed to stop btsnoop log capture: ", err)
	}
	// TODO(crbug/1189962): btsnoopCmd needs the fixture's context until test-scoped context is available.
	// This means we have to re-use the same testexec.Cmd between test runs, but they are not meant to be started more than once.
	// In order to restart it and successfully reuse the Cmd, we must set its Process field to nil.
	f.btsnoopCmd.Process = nil
	if err := fsutil.CopyFile(f.btsnoopTmpLogPath, filepath.Join(s.OutDir(), nearbycommon.BtsnoopLog)); err != nil {
		s.Fatal("Failed to save btsnoop logs from the device: ", err)
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
