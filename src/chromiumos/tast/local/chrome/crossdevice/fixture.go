// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crossdevice

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/android/adb"
	crossdevicecommon "chromiumos/tast/common/cros/crossdevice"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/crossdevice/crossdevicesettings"
	"chromiumos/tast/local/chrome/crossdevice/phonehub"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/logsaver"
	"chromiumos/tast/testing"
)

// FixtureOptions contains the options that we set for various crossDeviceFixture.
type FixtureOptions struct {
	allFeatures         bool // Whether or not to enable all cross device features.
	saveScreenRecording bool
	lockFixture         bool // Whether or not to lock the fixture preventing chrome from being torn down outside of fixture teardown.
	noSignIn            bool // Whether or not to sign in with the specified GAIA account. True will not skip OOBE.
}

// NewCrossDeviceOnboarded creates a fixture that logs in to CrOS, pairs it with an Android device,
// and ensures the features in the "Connected devices" section of OS Settings are ready to use (Smart Lock, Phone Hub, etc.).
// Note that crossdevice fixtures inherit from crossdeviceAndroidSetup.
func NewCrossDeviceOnboarded(opt FixtureOptions, fOpt chrome.OptionsCallback) testing.FixtureImpl {
	return &crossdeviceFixture{
		fOpt:                fOpt,
		allFeatures:         opt.allFeatures,
		saveScreenRecording: opt.saveScreenRecording,
		lockFixture:         opt.lockFixture,
		noSignIn:            opt.noSignIn,
	}
}

// Fixture runtime variables.
const (
	// These vars can be used from the command line when running tests locally to configure the tests to run on personal GAIA accounts.
	// Use these vars to log in with your own GAIA credentials on CrOS. The Android device should be signed in with the same account.
	customCrOSUsername = "cros_username"
	customCrOSPassword = "cros_password"
)

// postTestTimeout is the timeout for the fixture PostTest stage.
// We need a considerable amount of time to collect an Android bug report on failure.
const postTestTimeout = resetTimeout + BugReportDuration

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "crossdeviceOnboardedAllFeatures",
		Desc: "User is signed in (with GAIA) to CrOS and paired with an Android phone with all Cross Device features enabled",
		Contacts: []string{
			"kyleshima@chromium.org",
			"chromeos-sw-engprod@google.com",
		},
		Parent: "crossdeviceAndroidSetupPhoneHub",
		Impl: NewCrossDeviceOnboarded(FixtureOptions{true, true, true, false}, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return nil, nil
		}),
		Vars: []string{
			customCrOSUsername,
			customCrOSPassword,
			KeepStateVar,
		},
		SetUpTimeout:    10*time.Minute + BugReportDuration,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: postTestTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "crossdeviceOnboarded",
		Desc: "User is signed in (with GAIA) to CrOS and paired with an Android phone with default Cross Device features enabled",
		Contacts: []string{
			"kyleshima@chromium.org",
			"chromeos-sw-engprod@google.com",
		},
		Parent: "crossdeviceAndroidSetupSmartLock",
		Impl: NewCrossDeviceOnboarded(FixtureOptions{false, false, true, false}, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return nil, nil
		}),
		Vars: []string{
			customCrOSUsername,
			customCrOSPassword,
			KeepStateVar,
		},
		SetUpTimeout:    10*time.Minute + BugReportDuration,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: postTestTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "crossdeviceNoSignIn",
		Desc: "User is not signed in (with GAIA) to CrOS but fixture requires control of an Android phone. Does not skip OOBE",
		Contacts: []string{
			"kyleshima@chromium.org",
			"chromeos-sw-engprod@google.com",
		},
		Parent: "crossdeviceAndroidSetupPhoneHub",
		Impl: NewCrossDeviceOnboarded(FixtureOptions{false, false, true, true}, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return nil, nil
		}),
		Vars: []string{
			customCrOSUsername,
			customCrOSPassword,
			KeepStateVar,
			SignInProfileTestExtensionManifestKey,
		},
		SetUpTimeout:    10*time.Minute + BugReportDuration,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: postTestTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "crossdeviceOnboardedNoLock",
		Desc: "User is signed in (with GAIA) to CrOS and paired with an Android phone with default Cross Device features enabled. Doesn't lock the fixture before starting the test",
		Contacts: []string{
			"kyleshima@chromium.org",
			"chromeos-sw-engprod@google.com",
		},
		Parent: "crossdeviceAndroidSetupSmartLockLogin",
		Impl: NewCrossDeviceOnboarded(FixtureOptions{false, false, false, false}, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return nil, nil
		}),
		Vars: []string{
			customCrOSUsername,
			customCrOSPassword,
			KeepStateVar,
		},
		SetUpTimeout:    10*time.Minute + BugReportDuration,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: postTestTimeout,
	})

	// lacros fixtures
	testing.AddFixture(&testing.Fixture{
		Name: "lacrosCrossdeviceOnboardedAllFeatures",
		Desc: "User is signed in (with GAIA) to CrOS and paired with an Android phone with all Cross Device features enabled with lacros enabled",
		Contacts: []string{
			"kyleshima@chromium.org",
			"chromeos-sw-engprod@google.com",
		},
		Parent: "crossdeviceAndroidSetupPhoneHub",
		Impl: NewCrossDeviceOnboarded(FixtureOptions{true, true, true, false}, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return lacrosfixt.NewConfig().Opts()
		}),
		Vars: []string{
			customCrOSUsername,
			customCrOSPassword,
			KeepStateVar,
		},
		SetUpTimeout:    10*time.Minute + BugReportDuration,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: postTestTimeout,
	})
}

type crossdeviceFixture struct {
	fOpt                              chrome.OptionsCallback // Function to generate Chrome Options
	cr                                *chrome.Chrome
	tconn                             *chrome.TestConn
	kb                                *input.KeyboardEventWriter
	androidDevice                     *AndroidDevice
	androidAttributes                 *AndroidAttributes
	crosAttributes                    *crossdevicecommon.CrosAttributes
	btsnoopCmd                        *testexec.Cmd
	logMarker                         *logsaver.Marker // Marker for per-test log.
	allFeatures                       bool
	saveAndroidScreenRecordingOnError func(context.Context, func() bool) error
	saveScreenRecording               bool
	lockFixture                       bool
	noSignIn                          bool
	logcatStartTime                   adb.LogcatTimestamp
	downloadsPath                     string
}

// FixtData holds information made available to tests that specify this Fixture.
type FixtData struct {
	// Chrome is the running chrome instance.
	Chrome *chrome.Chrome

	// TestConn is a connection to the test extension.
	TestConn *chrome.TestConn

	// Connection to the lock screen test extension.
	LoginConn *chrome.TestConn

	// AndroidDevice is an object for interacting with the connected Android device's Multidevice Snippet.
	AndroidDevice *AndroidDevice

	// The credentials to be used on both chromebook and phone.
	Username string
	Password string

	// The options used to start Chrome sessions.
	ChromeOptions []chrome.Option
}

func (f *crossdeviceFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// Android device from parent fixture.
	androidDevice := s.ParentValue().(*FixtData).AndroidDevice
	f.androidDevice = androidDevice

	// Credentials to use (same as Android).
	crosUsername := s.ParentValue().(*FixtData).Username
	crosPassword := s.ParentValue().(*FixtData).Password

	// Allocate time for logging and saving a screenshot and bugreport in case of failure.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second+BugReportDuration)
	defer cancel()

	// Save logcat so we have Android logs even if fixture setup fails.
	startTime, err := androidDevice.Device.LatestLogcatTimestamp(ctx)
	if err != nil {
		s.Fatal("Failed to get latest logcat timestamp: ", err)
	}
	defer androidDevice.Device.DumpLogcatFromTimestamp(cleanupCtx, filepath.Join(s.OutDir(), "fixture_setup_logcat.txt"), startTime)
	defer androidDevice.DumpLogs(cleanupCtx, s.OutDir(), "fixture_setup_persistent_logcat.txt")

	// Set default chrome options.
	opts, err := f.fOpt(ctx, s)
	if err != nil {
		s.Fatal("Failed to obtain Chrome options: ", err)
	}

	tags := []string{
		"*nearby*=3",
		"*cryptauth*=3",
		"*device_sync*=3",
		"*multidevice*=3",
		"*secure_channel*=3",
		"*phonehub*=3",
		"*blue*=3",
		"ble_*=3",
	}
	opts = append(opts, chrome.ExtraArgs("--enable-logging", "--vmodule="+strings.Join(tags, ",")))
	opts = append(opts, chrome.EnableFeatures("PhoneHubCameraRoll", "SmartLockUIRevamp", "OobeQuickStart"))

	customUser, userOk := s.Var(customCrOSUsername)
	customPass, passOk := s.Var(customCrOSPassword)
	if userOk && passOk {
		s.Log("Logging in with user-provided credentials")
		crosUsername = customUser
		crosPassword = customPass
	} else {
		s.Log("Logging in with default GAIA credentials")
	}
	if f.noSignIn {
		opts = append(opts, chrome.DontSkipOOBEAfterLogin())
	} else {
		opts = append(opts, chrome.GAIALogin(chrome.Creds{User: crosUsername, Pass: crosPassword}))
	}
	if val, ok := s.Var(KeepStateVar); ok {
		b, err := strconv.ParseBool(val)
		if err != nil {
			s.Fatalf("Unable to convert %v var to bool: %v", KeepStateVar, err)
		}
		if b {
			opts = append(opts, chrome.KeepState())
		}
	}

	cr, err := chrome.New(
		ctx,
		opts...,
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	f.cr = cr

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	f.tconn = tconn
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "fixture")

	// Capture a bug report on the Android phone if any onboarding/setup fails.
	defer func() {
		if s.HasError() {
			if err := BugReport(ctx, androidDevice.Device, s.OutDir()); err != nil {
				s.Log("Failed to save Android bug report: ", err)
			}
		}
	}()

	// Capture btsnoop logs during fixture setup to have adequate logging during the onboarding phase.
	btsnoopCmd := bluetooth.StartBTSnoopLogging(ctx, filepath.Join(s.OutDir(), "crossdevice-fixture-btsnoop.log"))
	if err := btsnoopCmd.Start(); err != nil {
		s.Fatal("Failed to start btsnoop logging: ", err)
	}
	defer btsnoopCmd.Wait()
	defer btsnoopCmd.Kill()

	// Enable bluetooth debug logging.
	levels := bluetooth.LogVerbosity{
		Bluez:  true,
		Kernel: true,
	}
	if err := bluetooth.SetDebugLogLevels(ctx, levels); err != nil {
		return errors.Wrap(err, "failed to enable bluetooth debug logging")
	}

	// Phone and Chromebook will not be paired if we are not signed in to the Chromebook yet.
	if !f.noSignIn {
		// Sometimes during login the tcp connection to the snippet server on Android is lost.
		// If the Pair RPC fails, reconnect to the snippet server and try again.
		if err := f.PairWithAndroid(ctx, tconn, cr); err != nil {
			s.Fatal("Pairing with Android failed: ", err)
		}
		if f.allFeatures {
			// Wait for the "Smart Lock is turned on" notification to appear,
			// since it will cause Phone Hub to close if it's open before the notification pops up.
			if _, err := ash.WaitForNotification(ctx, tconn, 30*time.Second, ash.WaitTitleContains("Smart Lock is turned on")); err != nil {
				s.Log("Smart Lock notification did not appear after 30 seconds, proceeding anyways")
			}

			if err := phonehub.Enable(ctx, tconn, cr); err != nil {
				s.Fatal("Failed to enable Phone Hub: ", err)
			}
			if err := phonehub.Hide(ctx, tconn); err != nil {
				s.Fatal("Failed to hide Phone Hub after enabling it: ", err)
			}
			if err := androidDevice.EnablePhoneHubNotifications(ctx); err != nil {
				s.Fatal("Failed to enable Phone Hub notifications: ", err)
			}
		}
		if _, err := ash.WaitForNotification(ctx, tconn, 90*time.Second, ash.WaitTitleContains("Connected to")); err != nil {
			s.Fatal("Did not receive notification that Chromebook and Phone are paired")
		}
	}

	// Store Android attributes for reporting.
	androidAttributes, err := androidDevice.GetAndroidAttributes(ctx)
	if err != nil {
		s.Fatal("Failed to get Android attributes for reporting: ", err)
	}
	f.androidAttributes = androidAttributes

	// Store CrOS test metadata for reporting.
	crosAttributes, err := GetCrosAttributes(ctx, tconn, crosUsername)
	if err != nil {
		s.Fatal("Failed to get CrOS attributes for reporting: ", err)
	}
	f.crosAttributes = crosAttributes

	// Get the user's Download path for saving screen recordings.
	f.downloadsPath, err = cryptohome.DownloadsPath(ctx, f.cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Downloads path: ", err)
	}

	// Lock chrome after all Setup is complete so we don't block other fixtures.
	if f.lockFixture {
		chrome.Lock()
	}

	return &FixtData{
		Chrome:        cr,
		TestConn:      tconn,
		AndroidDevice: androidDevice,
		Username:      crosUsername,
		Password:      crosPassword,
		ChromeOptions: opts,
	}
}

func (f *crossdeviceFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if f.lockFixture {
		chrome.Unlock()
		if err := f.cr.Close(ctx); err != nil {
			s.Log("Failed to close Chrome connection: ", err)
		}
	}
	f.cr = nil
}
func (f *crossdeviceFixture) Reset(ctx context.Context) error {
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}
	return nil
}
func (f *crossdeviceFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	if err := saveDeviceAttributes(f.crosAttributes, f.androidAttributes, filepath.Join(s.OutDir(), "device_attributes.json")); err != nil {
		s.Error("Failed to save device attributes: ", err)
	}
	f.btsnoopCmd = bluetooth.StartBTSnoopLogging(s.TestContext(), filepath.Join(s.OutDir(), "crossdevice-btsnoop.log"))
	if err := f.btsnoopCmd.Start(); err != nil {
		s.Fatal("Failed to start btsnoop logging: ", err)
	}

	if f.logMarker != nil {
		s.Log("A log marker is already created but not cleaned up")
	}
	logMarker, err := logsaver.NewMarker(f.cr.LogFilename())
	if err == nil {
		f.logMarker = logMarker
	} else {
		s.Log("Failed to start the log saver: ", err)
	}

	timestamp, err := f.androidDevice.Device.LatestLogcatTimestamp(ctx)
	if err != nil {
		s.Fatal("Failed to get latest logcat timestamp: ", err)
	}
	f.logcatStartTime = timestamp

	if f.saveScreenRecording {
		if f.kb == nil {
			// Use virtual keyboard since uiauto.StartRecordFromKB assumes F5 is the overview key.
			kb, err := input.VirtualKeyboard(ctx)
			if err != nil {
				s.Fatal("Failed to setup keyboard for screen recording: ", err)
			}
			f.kb = kb
		}
		if err := uiauto.StartRecordFromKB(ctx, f.tconn, f.kb, f.downloadsPath); err != nil {
			s.Fatal("Failed to start screen recording on CrOS: ", err)
		}

		saveScreen, err := f.androidDevice.StartScreenRecording(s.TestContext(), "android-screen", s.OutDir())
		if err != nil {
			s.Fatal("Failed to start screen recording on Android: ", err)
		}
		f.saveAndroidScreenRecordingOnError = saveScreen
	}
}

func (f *crossdeviceFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if err := f.btsnoopCmd.Kill(); err != nil {
		s.Error("Failed to stop btsnoop log capture: ", err)
	}
	f.btsnoopCmd.Wait()
	f.btsnoopCmd = nil

	if f.logMarker != nil {
		if err := f.logMarker.Save(filepath.Join(s.OutDir(), "chrome.log")); err != nil {
			s.Log("Failed to store per-test log data: ", err)
		}
		f.logMarker = nil
	}

	// Restore connection to the ADB-over-WiFi device if it was lost during the test.
	// This is needed for Instant Tether tests that disable WiFi on the Chromebook which interrupts the ADB connection.
	if PhoneIP.Value() != "" && f.androidDevice.Device.IsConnected(ctx) != nil {
		s.Log("Connection to ADB device lost, restaring")
		device, err := AdbOverWifi(ctx)
		if err != nil {
			s.Fatal("Failed to re-initialize adb-over-wifi: ", err)
		}
		f.androidDevice.Device = device

		if err := f.androidDevice.ReconnectToSnippet(ctx); err != nil {
			s.Fatal("Failed to reconnect to the snippet: ", err)
		}
	}

	if err := f.androidDevice.Device.DumpLogcatFromTimestamp(ctx, filepath.Join(s.OutDir(), "crossdevice-logcat.txt"), f.logcatStartTime); err != nil {
		s.Fatal("Failed to save logcat logs from the test: ", err)
	}
	if err := f.androidDevice.DumpLogs(ctx, s.OutDir(), "crossdevice-persistent-logcat.txt"); err != nil {
		s.Fatal("Failed to save persistent logcat logs: ", err)
	}

	if f.saveScreenRecording {
		if err := f.saveAndroidScreenRecordingOnError(ctx, s.HasError); err != nil {
			s.Fatal("Failed to save Android screen recording: ", err)
		}
		f.saveAndroidScreenRecordingOnError = nil

		ui := uiauto.New(f.tconn)
		var crosRecordErr error
		if err := ui.Exists(uiauto.ScreenRecordStopButton)(ctx); err != nil {
			// Smart Lock tests automatically stop the screen recording when they lock the screen.
			// The screen recording should still exist though.
			crosRecordErr = uiauto.SaveRecordFromKBOnError(ctx, f.tconn, s.HasError, s.OutDir(), f.downloadsPath)
		} else {
			crosRecordErr = uiauto.StopRecordFromKBAndSaveOnError(ctx, f.tconn, s.HasError, s.OutDir(), f.downloadsPath)
		}
		if crosRecordErr != nil {
			s.Fatal("Failed to save CrOS screen recording: ", crosRecordErr)
		}
	}

	if s.HasError() {
		if err := BugReport(ctx, f.androidDevice.Device, s.OutDir()); err != nil {
			s.Error("Failed to save Android bug report: ", err)
		}
	}
}

// Verify that pairing between Android and Chromebook is successful.
func (f *crossdeviceFixture) PairWithAndroid(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) error {
	if err := f.androidDevice.Pair(ctx); err != nil {
		if err := f.androidDevice.ReconnectToSnippet(ctx); err != nil {
			return errors.Wrap(err, "failed to reconnect to the snippet server")
		}
		if err := f.androidDevice.Pair(ctx); err != nil {
			return errors.Wrap(err, "failed to connect the Android device to CrOS")
		}
	}
	if err := crossdevicesettings.WaitForConnectedDevice(ctx, tconn, cr); err != nil {
		return errors.Wrap(err, "failed waiting for the connected device to appear in OS settings")
	}
	return nil
}

// saveDeviceAttributes saves the CrOS and Android device attributes as a formatted JSON at the specified filepath.
func saveDeviceAttributes(crosAttrs *crossdevicecommon.CrosAttributes, androidAttrs *AndroidAttributes, filepath string) error {
	attributes := struct {
		CrOS    *crossdevicecommon.CrosAttributes
		Android *AndroidAttributes
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

// ConnectToWifi connects the chromebook to the Wifi network in its RF box.
func ConnectToWifi(ctx context.Context) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := testexec.CommandContext(ctx, "/usr/local/autotest/cros/scripts/wifi", "connect", "nearbysharing_1", "password").CombinedOutput(testexec.DumpLogOnError)
		if err != nil {
			if strings.Contains(string(out), "already connected") {
				testing.ContextLog(ctx, "Already connected to wifi network")
				return nil
			}
			return errors.Wrap(err, "failed to connect CrOS device to Wifi")
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second, Interval: 3 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to connect to wifi")
	}
	return nil
}
