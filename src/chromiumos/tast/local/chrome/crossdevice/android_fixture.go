// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crossdevice

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// NewCrossDeviceAndroid creates a fixture that sets up an Android device for crossdevice feature testing.
func NewCrossDeviceAndroid(feature Feature) testing.FixtureImpl {
	return &crossdeviceAndroidFixture{
		feature: feature,
	}
}

// resetTimeout is the timeout duration to trying reset of the current fixture.
const resetTimeout = 30 * time.Second

// Runtime variable names.
const (
	// These are the default GAIA credentials that will be used to sign in on Android for crossdevice tests.
	defaultCrossDeviceUsername = "crossdevice.username"
	defaultCrossDevicePassword = "crossdevice.password"

	// These are the default credentials used for Smart Lock.
	smartLockUsername = "crossdevice.smartLockUsername"
	smartLockPassword = "crossdevice.smartLockPassword"

	// These are alternative Smart Lock credentials used to test logging in.
	// We use two accounts so both features can be tested independently without worrying about colissions with other tests run in parallel.
	smartLockLoginUsername = "crossdevice.smartLockLoginUsername"
	smartLockLoginPassword = "crossdevice.smartLockLoginPassword"

	// Specify -var=skipAndroidLogin=true if the Android device is logged in to a personal account.
	// Otherwise we will attempt removing all Google accounts and adding a test account to the phone.
	// Adding/removing accounts requires ADB root access, so this will automatically be set to true if root is not available.
	skipAndroidLogin = "skipAndroidLogin"

	// Per RF box GAIA accounts.
	crossDevicePerBoxUsername1  = "crossdevice.PerBoxUser1"
	crossDevicePerBoxUsername2  = "crossdevice.PerBoxUser2"
	crossDevicePerBoxUsername3  = "crossdevice.PerBoxUser3"
	crossDevicePerBoxUsername4  = "crossdevice.PerBoxUser4"
	crossDevicePerBoxUsername5  = "crossdevice.PerBoxUser5"
	crossDevicePerBoxUsername6  = "crossdevice.PerBoxUser6"
	crossDevicePerBoxUsername7  = "crossdevice.PerBoxUser7"
	crossDevicePerBoxUsername8  = "crossdevice.PerBoxUser8"
	crossDevicePerBoxUsername9  = "crossdevice.PerBoxUser9"
	crossDevicePerBoxUsername10 = "crossdevice.PerBoxUser10"
	crossDevicePerBoxUsername11 = "crossdevice.PerBoxUser11"
	crossDevicePerBoxUsername12 = "crossdevice.PerBoxUser12"
	crossDevicePerBoxUsername13 = "crossdevice.PerBoxUser13"
	crossDevicePerBoxUsername14 = "crossdevice.PerBoxUser14"
	crossDevicePerBoxUsername15 = "crossdevice.PerBoxUser15"
	crossDevicePerBoxUsername16 = "crossdevice.PerBoxUser16"
	crossDevicePerBoxUsername17 = "crossdevice.PerBoxUser17"
	crossDevicePerBoxPassword   = "crossdevice.PerBoxPassword"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "crossdeviceAndroidSetupPhoneHub",
		Desc: "Set up Android device for CrOS crossdevice testing",
		Impl: NewCrossDeviceAndroid(Feature{Name: PhoneHub}),
		Data: []string{AccountUtilZip, MultideviceSnippetZipName},
		Contacts: []string{
			"kyleshima@chromium.org",
			"chromeos-sw-engprod@google.com",
		},
		Vars: []string{
			defaultCrossDeviceUsername,
			defaultCrossDevicePassword,
			skipAndroidLogin,
			crossDevicePerBoxUsername1,
			crossDevicePerBoxUsername2,
			crossDevicePerBoxUsername3,
			crossDevicePerBoxUsername4,
			crossDevicePerBoxUsername5,
			crossDevicePerBoxUsername6,
			crossDevicePerBoxUsername7,
			crossDevicePerBoxUsername8,
			crossDevicePerBoxUsername9,
			crossDevicePerBoxUsername10,
			crossDevicePerBoxUsername11,
			crossDevicePerBoxUsername12,
			crossDevicePerBoxUsername13,
			crossDevicePerBoxUsername14,
			crossDevicePerBoxUsername15,
			crossDevicePerBoxUsername16,
			crossDevicePerBoxUsername17,
			crossDevicePerBoxPassword,
		},
		SetUpTimeout:    3 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "crossdeviceAndroidSetupSmartLock",
		Desc: "Set up Android device for CrOS crossdevice testing of Smart Lock",
		Impl: NewCrossDeviceAndroid(Feature{Name: SmartLock, SubFeature: SmartLockUnlock}),
		Data: []string{AccountUtilZip, MultideviceSnippetZipName},
		Contacts: []string{
			"kyleshima@chromium.org",
			"chromeos-sw-engprod@google.com",
		},
		Vars: []string{
			smartLockUsername,
			smartLockPassword,
			skipAndroidLogin,
			crossDevicePerBoxUsername1,
			crossDevicePerBoxUsername2,
			crossDevicePerBoxUsername3,
			crossDevicePerBoxUsername4,
			crossDevicePerBoxUsername5,
			crossDevicePerBoxUsername6,
			crossDevicePerBoxUsername7,
			crossDevicePerBoxUsername8,
			crossDevicePerBoxUsername9,
			crossDevicePerBoxUsername10,
			crossDevicePerBoxUsername11,
			crossDevicePerBoxUsername12,
			crossDevicePerBoxUsername13,
			crossDevicePerBoxUsername14,
			crossDevicePerBoxUsername15,
			crossDevicePerBoxUsername16,
			crossDevicePerBoxUsername17,
			crossDevicePerBoxPassword,
		},
		SetUpTimeout:    3 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
	testing.AddFixture(&testing.Fixture{
		Name: "crossdeviceAndroidSetupSmartLockLogin",
		Desc: "Set up Android device for CrOS crossdevice testing of Smart Lock login",
		Impl: NewCrossDeviceAndroid(Feature{Name: SmartLock, SubFeature: SmartLockLogin}),
		Data: []string{AccountUtilZip, MultideviceSnippetZipName},
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Vars: []string{
			smartLockLoginUsername,
			smartLockLoginPassword,
			skipAndroidLogin,
			crossDevicePerBoxUsername1,
			crossDevicePerBoxUsername2,
			crossDevicePerBoxUsername3,
			crossDevicePerBoxUsername4,
			crossDevicePerBoxUsername5,
			crossDevicePerBoxUsername6,
			crossDevicePerBoxUsername7,
			crossDevicePerBoxUsername8,
			crossDevicePerBoxUsername9,
			crossDevicePerBoxUsername10,
			crossDevicePerBoxUsername11,
			crossDevicePerBoxUsername12,
			crossDevicePerBoxUsername13,
			crossDevicePerBoxUsername14,
			crossDevicePerBoxUsername15,
			crossDevicePerBoxUsername16,
			crossDevicePerBoxUsername17,
			crossDevicePerBoxPassword,
		},
		SetUpTimeout:    3 * time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		PreTestTimeout:  resetTimeout,
		PostTestTimeout: resetTimeout,
	})
}

type crossdeviceAndroidFixture struct {
	adbDevice     *adb.Device
	androidDevice *AndroidDevice
	feature       Feature
}

func (f *crossdeviceAndroidFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	accountUtilZip := s.DataPath(AccountUtilZip)
	snippetZip := s.DataPath(MultideviceSnippetZipName)

	// Set up adb, connect to the Android phone, and check if ADB root access is available.
	adbDevice, rooted, err := AdbSetup(ctx)
	if err != nil {
		s.Fatal("Failed to set up an adb device: ", err)
	}
	f.adbDevice = adbDevice
	// We want to ensure we have logs even if the Android device setup fails.
	fixtureLogcatPath := filepath.Join(s.OutDir(), "android_base_fixture_logcat.txt")
	defer adbDevice.DumpLogcat(ctx, fixtureLogcatPath)

	// Do some basic device set up like waking the screen and clearing logcat.
	if err := ConfigureDevice(ctx, adbDevice, rooted); err != nil {
		s.Fatal("Failed to prepare the Android device: ", err)
	}

	// Enable verbose logging for related modules.
	tags := []string{"ProximityAuth", "CryptauthV2", "NearbyConnections", "NearbyMediums"}
	if err := EnableVerboseLogging(ctx, adbDevice, rooted, tags...); err != nil {
		s.Fatal("Failed to enable verbose logs: ", err)
	}

	if err := OverrideFeatureFlags(ctx, adbDevice, f.feature); err != nil {
		s.Fatal("Failed to override required phenotype flags for feature ", f.feature.Name, ": ", err)
	}

	// Skip logging in to the test account on the Android device if specified in the runtime vars.
	// This lets you run the tests on a phone that's already signed in with your own account.
	loggedIn := false
	if val, ok := s.Var(skipAndroidLogin); ok {
		b, err := strconv.ParseBool(val)
		if err != nil {
			s.Fatal("Unable to convert skipAndroidLogin var to bool: ", err)
		}
		loggedIn = b
	}
	androidUsername, androidPassword, err := GetLoginCredentials(ctx, s, f.feature)
	if err != nil {
		s.Fatal("Failed to get login credentials: ", err)
	}
	androidUsername = s.RequiredVar(androidUsername)
	androidPassword = s.RequiredVar(androidPassword)

	if !loggedIn {
		if rooted {
			if err := GAIALogin(ctx, adbDevice, accountUtilZip, androidUsername, androidPassword); err != nil {
				s.Fatal("Failed to log in on the Android device: ", err)
			}
		} else {
			s.Fatal("Cannot log in on Android on an unrooted phone")
		}
	}

	// Prepare the Multidevice Snippet.
	androidDevice, err := NewAndroidDevice(ctx, adbDevice, snippetZip)
	if err != nil {
		s.Fatal("Failed to prepare connected Android device for Multidevice testing: ", err)
	}
	f.androidDevice = androidDevice
	return &FixtData{
		AndroidDevice: androidDevice,
		Username:      androidUsername,
		Password:      androidPassword,
	}
}
func (f *crossdeviceAndroidFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if f.androidDevice != nil {
		f.androidDevice.Cleanup(ctx)
	}
	if err := RemoveAccounts(ctx, f.adbDevice); err != nil {
		s.Log("Failed to remove accounts from the Android device: ", err)
	}
}
func (f *crossdeviceAndroidFixture) Reset(ctx context.Context) error                        { return nil }
func (f *crossdeviceAndroidFixture) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (f *crossdeviceAndroidFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

// GetLoginCredentials returns the correct credentials to use based on the Cross Device feature being tested.
func GetLoginCredentials(ctx context.Context, s *testing.FixtState, feature Feature) (string, string, error) {
	var username, password, ipaddress string

	// Choose the account to use based on the IP address of the chromebook.
	cmd := `ifconfig eth0 | grep "inet " | awk '{print $2}'`
	out, err := testexec.CommandContext(ctx, "sh", "-c", cmd).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Log("Failed to get IP Address of Chromebook: ", err)
		ipaddress = ""
	} else {
		ipaddress = strings.TrimSpace(string(out))
	}
	password = crossDevicePerBoxPassword
	switch ipaddress {
	// chromeos15-row3-metro2-unit3 (asurada|corsola)
	case "100.115.21.78", "100.115.21.79":
		username = crossDevicePerBoxUsername14
	// chromeos15-row3-metro4-unit3 (asurada|sentry)
	case "172.27.213.21", "172.27.213.22":
		username = crossDevicePerBoxUsername15
	// chromeos15-row3-metro1-unit2 (atlas|atlas)
	case "100.115.21.13", "100.115.21.14":
		username = crossDevicePerBoxUsername1
	// chromeos15-row3-metro3-unit3 (atlas|atlas)
	case "172.27.212.197", "172.27.212.198":
		username = crossDevicePerBoxUsername5
	// chromeos15-row3-metro3-unit1 (atlas|cherry)
	case "172.27.212.175", "172.27.212.176":
		username = crossDevicePerBoxUsername3
	// chromeos15-row3-metro1-unit3 (brya|hana)
	case "100.115.21.24", "100.115.21.30":
		username = crossDevicePerBoxUsername12
	// chromeos15-row3-metro1-unit1 (coral|sarien)
	case "100.115.21.2", "100.115.21.3":
		username = crossDevicePerBoxUsername9
	// chromeos15-row5-rack12-unit7 (dedede|herobrine)
	case "100.71.236.135", "100.71.236.141":
		username = crossDevicePerBoxUsername16
	// chromeos15-row3-metro4-unit1 (grunt|scarlet)
	case "172.27.212.253", "172.27.212.254":
		username = crossDevicePerBoxUsername7
	// chromeos15-row3-metro3-unit2 (grunt|soraka)
	case "172.27.212.186", "172.27.212.187":
		username = crossDevicePerBoxUsername4
	// chromeos15-row3-metro1-unit4 (guybrush|kevin)
	case "100.115.21.34", "100.115.21.40":
		username = crossDevicePerBoxUsername11
	// chromeos15-row3-metro2-unit2 (guybrush|kukui)
	case "100.115.21.67", "100.115.21.68":
		username = crossDevicePerBoxUsername17
	// chromeos15-row3-metro4-unit2 (hatch|octopus)
	case "172.27.213.10", "172.27.213.11":
		username = crossDevicePerBoxUsername8
	// chromeos15-row3-metro2-unit4 (jacuzzi|jacuzzi)
	case "100.115.21.89", "100.115.21.90":
		username = crossDevicePerBoxUsername2
	// chromeos15-row3-metro4-unit4 (kukui|puff)
	case "172.27.213.32", "172.27.213.33":
		username = crossDevicePerBoxUsername10
	// chromeos15-row3-metro3-unit4 (octopus|strongbad)
	case "172.27.212.208", "172.27.212.209":
		username = crossDevicePerBoxUsername6
	// chromeos15-row3-metro2-unit1 (volteer|zork)
	case "100.115.21.56", "100.115.21.57":
		username = crossDevicePerBoxUsername13
	default:
		switch feature.Name {
		case SmartLock:
			switch feature.SubFeature {
			case SmartLockUnlock:
				username = smartLockUsername
				password = smartLockPassword
			case SmartLockLogin:
				username = smartLockLoginUsername
				password = smartLockLoginPassword
			default:
				return "", "", errors.New("unknown subfeature specified for Smart Lock")
			}
		case PhoneHub:
			username = defaultCrossDeviceUsername
			password = defaultCrossDevicePassword
		default:
			return "", "", errors.New("unknown Cross Device feature specified")
		}
	}
	s.Logf("GAIA account chosen: %s", username)
	return username, password, nil

}

// OverrideFeatureFlags overrides the required Phenotype flags for the given cross device feature.
func OverrideFeatureFlags(ctx context.Context, adbDevice *adb.Device, feature Feature) error {
	switch feature.Name {
	case PhoneHub:
		// These flags need to be overridden before logging into the account so that they can have the desired values during CryptAuth enrollment.
		if err := adbDevice.OverridePhenotypeFlag(ctx, "com.google.android.gms.auth.proximity", "PhoneHub__enable_camera_roll", "true", "boolean"); err != nil {
			return errors.Wrap(err, "failed to override required flag for Phone Hub")
		}
		if err := adbDevice.OverridePhenotypeFlag(ctx, "com.google.android.gms.auth.proximity", "PhoneHub__set_camera_roll_host_supported", "true", "boolean"); err != nil {
			return errors.Wrap(err, "failed to override required flag for Phone Hub")
		}
		if err := adbDevice.OverridePhenotypeFlag(ctx, "com.google.android.gms.auth.proximity", "PhoneHub__enable_feature_setup_request", "true", "boolean"); err != nil {
			return errors.Wrap(err, "failed to override required flag for Phone Hub")
		}
		return nil
	case SmartLock:
		// These flags need to be overridden to ensure Nearby Share doesn't tear down Smart Lock's GATT connection when the phone's screen is unlocked (b/219981726).
		if err := adbDevice.OverridePhenotypeFlag(ctx, "com.google.android.gms.nearby", "connections_allow_control_ble_gatt_connection_in_advertising_option", "true", "boolean"); err != nil {
			return errors.Wrap(err, "failed to override required flag for SmartLock")
		}
		if err := adbDevice.OverridePhenotypeFlag(ctx, "com.google.android.gms.nearby", "sharing_enable_self_share_background_advertising", "true", "boolean"); err != nil {
			return errors.Wrap(err, "failed to override required flag for SmartLock")
		}
		return nil
	default:
		return nil
	}
}
