// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"chromiumos/tast/local/chrome/internal/config"
	"chromiumos/tast/local/cryptohome"
)

const (
	// DefaultUser contains the email address used to log into Chrome when authentication credentials are not supplied.
	DefaultUser = config.DefaultUser

	// DefaultPass contains the password we use to log into the DefaultUser account.
	DefaultPass = config.DefaultPass
)

// Option is a self-referential function can be used to configure Chrome.
// See https://commandcenter.blogspot.com.au/2014/01/self-referential-functions-and-design.html
// for details about this pattern.
type Option = config.Option

// EnableWebAppInstall returns an Option that can be passed to enable web app auto-install after user login.
// By default web app auto-install is disabled to reduce network traffic in test environment.
// See https://crbug.com/1076660 for more details.
func EnableWebAppInstall() Option {
	return func(cfg *config.Config) { cfg.InstallWebApp = true }
}

// EnableLoginVerboseLogs returns an Option that enables verbose logging for some login-related files.
func EnableLoginVerboseLogs() Option {
	return func(cfg *config.Config) { cfg.EnableLoginVerboseLogs = true }
}

// VKEnabled returns an Option that force enable virtual keyboard.
// VKEnabled option appends "--enable-virtual-keyboard" to chrome initialization and also checks VK connection after user login.
// Note: This option can not be used by ARC tests as some boards block VK background from presence.
func VKEnabled() Option {
	return func(cfg *config.Config) { cfg.VKEnabled = true }
}

// Auth returns an Option that can be passed to New to configure the login credentials used by Chrome.
// Please do not check in real credentials to public repositories when using this in conjunction with GAIALogin.
func Auth(user, pass, gaiaID string) Option {
	return func(cfg *config.Config) {
		cfg.User = user
		cfg.Pass = pass
		cfg.GAIAID = gaiaID
	}
}

// Contact returns an Option that can be passed to New to configure the contact email used by Chrome for
// cross account challenge (go/ota-security). Please do not check in real credentials to public repositories
// when using this in conjunction with GAIALogin.
func Contact(contact string) Option {
	return func(cfg *config.Config) {
		cfg.Contact = contact
	}
}

// ParentAuth returns an Option that can be passed to New to configure the login credentials of a parent user.
// If the GAIA account specified by Auth is a supervised child user, this credential is used to go through the unicorn login flow.
// Please do not check in real credentials to public repositories when using this in conjunction with GAIALogin.
func ParentAuth(parentUser, parentPass string) Option {
	return func(cfg *config.Config) {
		cfg.ParentUser = parentUser
		cfg.ParentPass = parentPass
	}
}

// KeepState returns an Option that can be passed to New to preserve the state such as
// files under /home/chronos and the user's existing cryptohome (if any) instead of
// wiping them before logging in.
func KeepState() Option {
	return func(cfg *config.Config) { cfg.KeepState = true }
}

// DeferLogin returns an option that instructs chrome.New to return before logging into a session.
// After successful return of chrome.New, you can call ContinueLogin to continue login.
func DeferLogin() Option {
	return func(cfg *config.Config) { cfg.DeferLogin = true }
}

// GAIALogin returns an Option that can be passed to New to perform a real GAIA-based login rather
// than the default fake login.
func GAIALogin() Option {
	return func(cfg *config.Config) { cfg.LoginMode = config.GAIALogin }
}

// NoLogin returns an Option that can be passed to New to avoid logging in.
// Chrome is still restarted with testing-friendly behavior.
func NoLogin() Option {
	return func(cfg *config.Config) { cfg.LoginMode = config.NoLogin }
}

// GuestLogin returns an Option that can be passed to New to log in as guest
// user.
func GuestLogin() Option {
	return func(cfg *config.Config) {
		cfg.LoginMode = config.GuestLogin
		cfg.User = cryptohome.GuestUser
	}
}

// DontSkipOOBEAfterLogin returns an Option that can be passed to stay in OOBE after user login.
func DontSkipOOBEAfterLogin() Option {
	return func(cfg *config.Config) {
		cfg.SkipOOBEAfterLogin = false
	}
}

// Region returns an Option that can be passed to New to set the region deciding
// the locale used in the OOBE screen and the user sessions. region is a
// two-letter code such as "us", "fr", or "ja".
func Region(region string) Option {
	return func(cfg *config.Config) {
		cfg.Region = region
	}
}

// ProdPolicy returns an option that can be passed to New to let the device do a
// policy fetch upon login. By default, policies are not fetched.
// The default Device Management service is used.
func ProdPolicy() Option {
	return func(cfg *config.Config) {
		cfg.PolicyEnabled = true
		cfg.DMSAddr = ""
	}
}

// DMSPolicy returns an option that can be passed to New to tell the device to fetch
// policies from the policy server at the given url. By default policies are not
// fetched.
func DMSPolicy(url string) Option {
	return func(cfg *config.Config) {
		cfg.PolicyEnabled = true
		cfg.DMSAddr = url
	}
}

// EnterpriseEnroll returns an Option that can be passed to New to enable Enterprise
// Enrollment
func EnterpriseEnroll() Option {
	return func(cfg *config.Config) { cfg.Enroll = true }
}

// ARCDisabled returns an Option that can be passed to New to disable ARC.
func ARCDisabled() Option {
	return func(cfg *config.Config) { cfg.ARCMode = config.ARCDisabled }
}

// ARCEnabled returns an Option that can be passed to New to enable ARC (without Play Store)
// for the user session with mock GAIA account.
func ARCEnabled() Option {
	return func(cfg *config.Config) { cfg.ARCMode = config.ARCEnabled }
}

// ARCSupported returns an Option that can be passed to New to allow to enable ARC with Play Store gaia opt-in for the user
// session with real GAIA account.
// In this case ARC is not launched by default and is required to be launched by user policy or from UI.
func ARCSupported() Option {
	return func(cfg *config.Config) { cfg.ARCMode = config.ARCSupported }
}

// RestrictARCCPU returns an Option that can be passed to New which controls whether
// to let Chrome use CGroups to limit the CPU time of ARC when in the background.
// Most ARC-related tests should not pass this option.
func RestrictARCCPU() Option {
	return func(cfg *config.Config) { cfg.RestrictARCCPU = true }
}

// CrashNormalMode tells the crash handling system to act like it would on a
// real device. If this option is not used, the Chrome instances created by this package
// will skip calling crash_reporter and write any dumps into /home/chronos/crash directly
// from breakpad. This option restores the normal behavior of calling crash_reporter.
func CrashNormalMode() Option {
	return func(cfg *config.Config) { cfg.BreakpadTestMode = false }
}

// ExtraArgs returns an Option that can be passed to New to append additional arguments to Chrome's command line.
func ExtraArgs(args ...string) Option {
	return func(cfg *config.Config) { cfg.ExtraArgs = append(cfg.ExtraArgs, args...) }
}

// EnableFeatures returns an Option that can be passed to New to enable specific features in Chrome.
func EnableFeatures(features ...string) Option {
	return func(cfg *config.Config) { cfg.EnableFeatures = append(cfg.EnableFeatures, features...) }
}

// DisableFeatures returns an Option that can be passed to New to disable specific features in Chrome.
func DisableFeatures(features ...string) Option {
	return func(cfg *config.Config) { cfg.DisableFeatures = append(cfg.DisableFeatures, features...) }
}

// UnpackedExtension returns an Option that can be passed to New to make Chrome load an unpacked
// extension in the supplied directory.
// Ownership of the extension directory and its contents may be modified by New.
func UnpackedExtension(dir string) Option {
	return func(cfg *config.Config) { cfg.ExtraExtDirs = append(cfg.ExtraExtDirs, dir) }
}

// LoadSigninProfileExtension loads the test extension which is allowed to run in the signin profile context.
// Private manifest key should be passed (see ui.SigninProfileExtension for details).
func LoadSigninProfileExtension(key string) Option {
	return func(cfg *config.Config) { cfg.SigninExtKey = key }
}
