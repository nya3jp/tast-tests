// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"encoding/hex"
	"math/rand"
	"time"

	"chromiumos/tast/local/chrome/internal/config"
	"chromiumos/tast/local/cryptohome"
)

const (
	// DefaultUser contains the email address used to log into Chrome when authentication credentials are not supplied.
	DefaultUser = config.DefaultUser

	// DefaultPass contains the password we use to log into the DefaultUser account.
	DefaultPass = config.DefaultPass
)

// Creds contains credentials to log into a Chrome user session.
type Creds = config.Creds

// Option is a self-referential function can be used to configure Chrome.
// See https://commandcenter.blogspot.com.au/2014/01/self-referential-functions-and-design.html
// for details about this pattern.
type Option = config.Option

// EnableWebAppInstall returns an Option that can be passed to enable web app auto-install after user login.
// By default web app auto-install is disabled to reduce network traffic in test environment.
// See https://crbug.com/1076660 for more details.
func EnableWebAppInstall() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.InstallWebApp = true
		return nil
	}
}

// EnableLoginVerboseLogs returns an Option that enables verbose logging for some login-related files.
func EnableLoginVerboseLogs() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.EnableLoginVerboseLogs = true
		return nil
	}
}

// VKEnabled returns an Option that force enable virtual keyboard.
// VKEnabled option appends "--enable-virtual-keyboard" to chrome initialization and also checks VK connection after user login.
// Note: This option can not be used by ARC tests as some boards block VK background from presence.
func VKEnabled() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.VKEnabled = true
		return nil
	}
}

// KeepState returns an Option that can be passed to New to preserve the state such as
// files under /home/chronos and the user's existing cryptohome (if any) instead of
// wiping them before logging in.
func KeepState() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.KeepState = true
		return nil
	}
}

// DisablePolicyKeyVerification returns an Option that disables the policy key
// verification in Chrome.
func DisablePolicyKeyVerification() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.DisablePolicyKeyVerification = true
		return nil
	}
}

// DeferLogin returns an option that instructs chrome.New to return before logging into a session.
// After successful return of chrome.New, you can call ContinueLogin to continue login.
func DeferLogin() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.DeferLogin = true
		return nil
	}
}

// GAIALogin returns an Option that can be passed to New to perform a real
// GAIA-based login rather than the default fake login.
func GAIALogin(creds Creds) Option {
	return func(cfg *config.MutableConfig) error {
		cfg.LoginMode = config.GAIALogin
		cfg.Creds = creds
		return nil
	}
}

// UseSandboxGaia returns an Option that can be passed to New to instruct use
// the sandbox instance of Gaia.
// NOTE: Only works with the Gaia-based login options.
func UseSandboxGaia() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.UseSandboxGaia = true
		return nil
	}
}

var random = rand.New(rand.NewSource(time.Now().UnixNano()))

// GAIALoginPool returns an Option that can be passed to New to perform a real
// GAIA-based login with a pool of GAIA account credentials.
//
// creds is a string containing multiple credentials separated by newlines:
//
//	user1:pass1
//	user2:pass2
//	user3:pass3
//	...
//
// This option randomly picks one credentials. A chosen one is written to
// logs in chrome.New, as well as available via Chrome.Creds.
func GAIALoginPool(creds string) Option {
	return func(cfg *config.MutableConfig) error {
		creds, err := PickRandomCreds(creds)
		if err != nil {
			return err
		}
		cfg.LoginMode = config.GAIALogin
		cfg.Creds = creds
		return nil
	}
}

// PickRandomCreds randomly picks one credentials from the passed string. For
// the format details refer to the GAIALoginPool option documentation.
func PickRandomCreds(creds string) (Creds, error) {
	var result Creds
	cs, err := config.ParseCreds(creds)
	if err != nil {
		return result, err
	}
	result = cs[random.Intn(len(cs))]
	return result, nil
}

// FakeLogin returns an Option that can be passed to New to perform a fake
// login, which skips credential verifications with GAIA servers.
//
// When no login option is specified, the default is to perform a fake login
// with the default credentials (DefaultUser, DefaultPass). Thus you rarely
// need to specify this option. An example use case is to work with two or more
// fake accounts.
func FakeLogin(creds Creds) Option {
	return func(cfg *config.MutableConfig) error {
		cfg.LoginMode = config.FakeLogin
		cfg.Creds = creds
		return nil
	}
}

// NoLogin returns an Option that can be passed to New to avoid logging in.
// Chrome is still restarted with testing-friendly behavior.
func NoLogin() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.LoginMode = config.NoLogin
		return nil
	}
}

// MGSUser returns an option that can be passed to New to set the managed
// guest session account ID correctly.
func MGSUser(accountID string) Option {
	// The ID as it is generated by DeviceLocalAccount in Chrome
	// (https://source.chromium.org/chromium/chromium/src/+/main:chrome/browser/ash/policy/core/device_local_account.cc).
	generatedID := hex.EncodeToString([]byte(accountID)) + "@public-accounts.device-local.localhost"
	return func(cfg *config.MutableConfig) error {
		cfg.Creds = config.Creds{User: generatedID}
		return nil
	}
}

// GuestLogin returns an Option that can be passed to New to log in as guest
// user.
func GuestLogin() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.LoginMode = config.GuestLogin
		cfg.Creds = config.Creds{User: cryptohome.GuestUser}
		return nil
	}
}

// KeepEnrollment returns an Option that can be passed to New to ensure
// enrollment is not wiped and Chrome is ready for device policy testing.
func KeepEnrollment() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.KeepOwnership = true
		return nil
	}
}

// EphemeralUser returns an Option that can be passed to New to validate user
// mount as ephemeral, which is permanent by default.
func EphemeralUser() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.EphemeralUser = true
		return nil
	}
}

// DontSkipOOBEAfterLogin returns an Option that can be passed to stay in OOBE after user login.
func DontSkipOOBEAfterLogin() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.SkipOOBEAfterLogin = false
		return nil
	}
}

// DontWaitForCryptohome returns an Option that can be passed to skip cryptohome mount wait after user login.
func DontWaitForCryptohome() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.WaitForCryptohome = false
		return nil
	}
}

// CustomLoginTimeout allows setting a custom timeout for login.
func CustomLoginTimeout(timeout time.Duration) Option {
	return func(cfg *config.MutableConfig) error {
		cfg.CustomLoginTimeout = timeout.Nanoseconds()
		return nil
	}
}

// Region returns an Option that can be passed to New to set the region deciding
// the locale used in the OOBE screen and the user sessions. region is a
// two-letter code such as "us", "fr", or "ja".
func Region(region string) Option {
	return func(cfg *config.MutableConfig) error {
		cfg.Region = region
		return nil
	}
}

// ProdPolicy returns an option that can be passed to New to let the device do a
// policy fetch upon login. By default, policies are not fetched.
// The default Device Management service is used.
func ProdPolicy() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.PolicyEnabled = true
		cfg.DMSAddr = ""
		return nil
	}
}

// DMSPolicy returns an option that can be passed to New to tell the device to fetch
// policies from the policy server at the given url. By default policies are not
// fetched.
func DMSPolicy(url string) Option {
	return func(cfg *config.MutableConfig) error {
		cfg.PolicyEnabled = true
		cfg.DMSAddr = url
		return nil
	}
}

// EncryptedReportingAddr returns an option that can be passed to New to tell the device
// to report Chrome real-time events to the Encrypted Reporting Server at the given url.
// By default encrypted real-time event reporting is not enabled, and when enabled,
// Chrome reports event to the Prod Reporting Server at
// chromereporting-pa.googleapis.com.
func EncryptedReportingAddr(url string) Option {
	return func(cfg *config.MutableConfig) error {
		cfg.PolicyEnabled = true
		cfg.EncryptedReportingAddr = url
		return nil
	}
}

// GAIAEnterpriseEnroll returns an Option that can be passed to New to enable Enterprise
// Enrollment before login.
func GAIAEnterpriseEnroll(creds Creds) Option {
	return func(cfg *config.MutableConfig) error {
		cfg.EnrollMode = config.GAIAEnroll
		cfg.EnrollmentCreds = creds
		return nil
	}
}

// GAIAZTEEnterpriseEnroll returns an Option that can be passed to New to enable Enterprise
// Enrollment before login.
func GAIAZTEEnterpriseEnroll() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.EnrollMode = config.GAIAZTEEnroll
		return nil
	}
}

// FakeEnterpriseEnroll returns an Option that can be passed to New to enable Enterprise
// Enrollment with a fake local device management server before login.
func FakeEnterpriseEnroll(creds Creds) Option {
	return func(cfg *config.MutableConfig) error {
		cfg.EnrollMode = config.FakeEnroll
		cfg.EnrollmentCreds = creds
		cfg.DisablePolicyKeyVerification = true
		return nil
	}
}

// ARCDisabled returns an Option that can be passed to New to disable ARC.
func ARCDisabled() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.ARCMode = config.ARCDisabled
		return nil
	}
}

// ARCEnabled returns an Option that can be passed to New to enable ARC (without Play Store)
// for the user session with mock GAIA account.
func ARCEnabled() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.ARCMode = config.ARCEnabled
		return nil
	}
}

// HugePagesEnabled returns an Option that can be passed to New to enable huge pages within a
// session's ARCVM instance. Has no effect on boards running Android P or if ARCEnabled is not
// also passed to New.
func HugePagesEnabled() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.ARCUseHugePages = true
		return nil
	}
}

// ARCSupported returns an Option that can be passed to New to allow to enable ARC with Play Store gaia opt-in for the user
// session with real GAIA account.
// In this case ARC is not launched by default and is required to be launched by user policy or from UI.
func ARCSupported() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.ARCMode = config.ARCSupported
		return nil
	}
}

// UnRestrictARCCPU returns an Option that can be passed to New which controls whether
// to not let Chrome use CGroups to limit the CPU time of ARC when in the background.
// Most ARC-related tests should pass this option.
func UnRestrictARCCPU() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.UnRestrictARCCPU = true
		return nil
	}
}

// SkipForceOnlineSignInForTesting disables online sign-in enforcement in tast
// tests, which allows to run ChromeOS login in a loop in a stable way.
func SkipForceOnlineSignInForTesting() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.SkipForceOnlineSignInForTesting = true
		return nil
	}
}

// EnableRestoreTabs returns an Option that can be passed to New which controls whether
// to let Chrome use CGroups to limit the CPU time of ARC when in the background.
// Most ARC-related tests should not pass this option.
func EnableRestoreTabs() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.EnableRestoreTabs = true
		return nil
	}
}

// CrashNormalMode tells the crash handling system to act like it would on a
// real device. If this option is not used, the Chrome instances created by this package
// will skip calling crash_reporter and write any dumps into /home/chronos/crash directly
// from breakpad. This option restores the normal behavior of calling crash_reporter.
func CrashNormalMode() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.BreakpadTestMode = false
		return nil
	}
}

// ExtraArgs returns an Option that can be passed to New to append additional arguments to Chrome's command line.
func ExtraArgs(args ...string) Option {
	return func(cfg *config.MutableConfig) error {
		cfg.ExtraArgs = append(cfg.ExtraArgs, args...)
		return nil
	}
}

// LacrosExtraArgs returns an Option that can be passed to New to append additional arguments to Lacros Chrome's command line.
func LacrosExtraArgs(args ...string) Option {
	return func(cfg *config.MutableConfig) error {
		cfg.LacrosExtraArgs = append(cfg.LacrosExtraArgs, args...)
		return nil
	}
}

// EnableFeatures returns an Option that can be passed to New to enable specific features in Chrome.
func EnableFeatures(features ...string) Option {
	return func(cfg *config.MutableConfig) error {
		cfg.EnableFeatures = append(cfg.EnableFeatures, features...)
		return nil
	}
}

// LacrosEnableFeatures returns an Option that can be passed to New to enable specific features in Lacros Chrome.
func LacrosEnableFeatures(features ...string) Option {
	return func(cfg *config.MutableConfig) error {
		cfg.LacrosEnableFeatures = append(cfg.LacrosEnableFeatures, features...)
		return nil
	}
}

// DisableFeatures returns an Option that can be passed to New to disable specific features in Chrome.
func DisableFeatures(features ...string) Option {
	return func(cfg *config.MutableConfig) error {
		cfg.DisableFeatures = append(cfg.DisableFeatures, features...)
		return nil
	}
}

// LacrosDisableFeatures returns an Option that can be passed to New to disable specific features in Lacros Chrome.
func LacrosDisableFeatures(features ...string) Option {
	return func(cfg *config.MutableConfig) error {
		cfg.LacrosDisableFeatures = append(cfg.LacrosDisableFeatures, features...)
		return nil
	}
}

// UnpackedExtension returns an Option that can be passed to New to make Chrome load an unpacked
// extension in the supplied directory.
// The specified directory is copied to a different location before loading, so modifications to
// the directory do not take effect after starting Chrome.
func UnpackedExtension(dir string) Option {
	return func(cfg *config.MutableConfig) error {
		cfg.ExtraExtDirs = append(cfg.ExtraExtDirs, dir)
		return nil
	}
}

// LacrosUnpackedExtension is similar to UnpackedExtension, but for lacros-chrome.
func LacrosUnpackedExtension(dir string) Option {
	return func(cfg *config.MutableConfig) error {
		cfg.LacrosExtraExtDirs = append(cfg.LacrosExtraExtDirs, dir)
		return nil
	}
}

// LoadSigninProfileExtension loads the test extension which is allowed to run in the signin profile context.
// Private manifest key should be passed (see ui.SigninProfileExtension for details).
func LoadSigninProfileExtension(key string) Option {
	return func(cfg *config.MutableConfig) error {
		cfg.SigninExtKey = key
		return nil
	}
}

// TryReuseSession returns an Option that can be passed to New to make Chrome to reuse the existing
// login session from same user.
// Session will be re-used when Chrome configurations are compatible between two sessions.
// For noLogin mode and deferLogin option, session will not be re-used.
// If the existing session cannot be reused, a new Chrome session will be restarted.
func TryReuseSession() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.TryReuseSession = true
		return nil
	}
}

// RemoveNotification returns an Option that can be passed to New to make Chrome to
// remove or keep the notifications after login.
func RemoveNotification(remove bool) Option {
	return func(cfg *config.MutableConfig) error {
		cfg.RemoveNotification = remove
		return nil
	}
}

// HideCrashRestoreBubble returns an Option that can be passed to New to make Chrome to
// to skip "Chrome did not shut down correctly" bubble
func HideCrashRestoreBubble() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.HideCrashRestoreBubble = true
		return nil
	}
}

// ForceLaunchBrowser returns an Option that forces FullRestoreService to launch browser.
// ForceLaunchBrowser option appends "--force-launch-browser" to chrome initialization.
func ForceLaunchBrowser() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.ForceLaunchBrowser = true
		return nil
	}
}

// TestExtOAuthClientID returns an Option that provides an OAuth Client ID
// to the generated test extension.
func TestExtOAuthClientID(clientID string) Option {
	return func(cfg *config.MutableConfig) error {
		cfg.TestExtOAuthClientID = clientID
		return nil
	}
}

// EnableHIDScreenOnOOBE returns an Option that can be passed to New to keep the HID
// Detection screen and not forcibly skip it.
// For testing purposes, by default, we append "--disable-hid-detection-on-oobe" to
// chrome initialization except when this option is enabled.
func EnableHIDScreenOnOOBE() Option {
	return func(cfg *config.MutableConfig) error {
		cfg.EnableHIDScreenOnOOBE = true
		return nil
	}
}
