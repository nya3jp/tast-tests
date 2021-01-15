// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/session"
)

const (
	// DefaultUser contains the email address used to log into Chrome when authentication credentials are not supplied.
	DefaultUser = "testuser@gmail.com"

	// DefaultPass contains the password we use to log into the DefaultUser account.
	DefaultPass   = "testpass"
	defaultGaiaID = "gaia-id"
)

// arcMode describes the mode that ARC should be put into.
type arcMode int

const (
	arcDisabled arcMode = iota
	arcEnabled
	arcSupported // ARC is supported and can be launched by user policy
)

// loginMode describes the user mode for the login.
type loginMode int

const (
	noLogin    loginMode = iota // restart Chrome but don't log in
	fakeLogin                   // fake login with no authentication
	gaiaLogin                   // real network-based login using GAIA backend
	guestLogin                  // sign in as ephemeral guest user
)

// authType describes the type of authentication to be used in GAIA.
type authType string

const (
	unknownAuth  authType = ""         // cannot determine the authentication type
	passwordAuth authType = "password" // password based authentication
	contactAuth  authType = "contact"  // contact email approval based authentication
)

// config contains configurations for chrome.Chrome instance as requested by
// options to chrome.New.
//
// This is an immutable struct. Its fields must not be altered outside of Option
// and newConfig.
type config struct {
	user, pass, gaiaID, contact string // login credentials
	normalizedUser              string // user with domain added, periods removed, etc.
	parentUser, parentPass      string // unicorn parent login credentials
	keepState                   bool
	deferLogin                  bool
	loginMode                   loginMode
	enableLoginVerboseLogs      bool // enable verbose logging in some login related files
	vkEnabled                   bool
	skipOOBEAfterLogin          bool // skip OOBE post user login
	installWebApp               bool // auto install essential apps after user login
	region                      string
	policyEnabled               bool   // flag to enable policy fetch
	dmsAddr                     string // Device Management URL, or empty if using default
	enroll                      bool   // whether device should be enrolled
	arcMode                     arcMode
	restrictARCCPU              bool // a flag to control cpu restrictions on ARC

	// If breakpadTestMode is true, tell Chrome's breakpad to always write
	// dumps directly to a hardcoded directory.
	breakpadTestMode bool
	extraArgs        []string
	enableFeatures   []string
	disableFeatures  []string

	extraExtDirs []string // directories containing all extra unpacked extensions to load
	signinExtKey string   // private key for signin profile test extension manifest
}

// newConfig constructs config from a list of options given to chrome.New.
func newConfig(opts []Option) (*config, error) {
	cfg := &config{
		user:                   DefaultUser,
		pass:                   DefaultPass,
		gaiaID:                 defaultGaiaID,
		keepState:              false,
		loginMode:              fakeLogin,
		vkEnabled:              false,
		skipOOBEAfterLogin:     true,
		enableLoginVerboseLogs: false,
		installWebApp:          false,
		region:                 "us",
		policyEnabled:          false,
		enroll:                 false,
		breakpadTestMode:       true,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// TODO(rrsilva, crbug.com/1109176) - Disable login-related verbose logging
	// in all tests once the issue is solved.
	EnableLoginVerboseLogs()(cfg)

	// This works around https://crbug.com/358427.
	if cfg.loginMode == gaiaLogin {
		var err error
		if cfg.normalizedUser, err = session.NormalizeEmail(cfg.user, true); err != nil {
			return nil, errors.Wrapf(err, "failed to normalize email %q", cfg.user)
		}
	} else {
		cfg.normalizedUser = cfg.user
	}

	return cfg, nil
}

// Option is a self-referential function can be used to configure Chrome.
// See https://commandcenter.blogspot.com.au/2014/01/self-referential-functions-and-design.html
// for details about this pattern.
type Option func(cfg *config)

// EnableWebAppInstall returns an Option that can be passed to enable web app auto-install after user login.
// By default web app auto-install is disabled to reduce network traffic in test environment.
// See https://crbug.com/1076660 for more details.
func EnableWebAppInstall() Option {
	return func(cfg *config) { cfg.installWebApp = true }
}

// EnableLoginVerboseLogs returns an Option that enables verbose logging for some login-related files.
func EnableLoginVerboseLogs() Option {
	return func(cfg *config) { cfg.enableLoginVerboseLogs = true }
}

// VKEnabled returns an Option that force enable virtual keyboard.
// VKEnabled option appends "--enable-virtual-keyboard" to chrome initialization and also checks VK connection after user login.
// Note: This option can not be used by ARC tests as some boards block VK background from presence.
func VKEnabled() Option {
	return func(cfg *config) { cfg.vkEnabled = true }
}

// Auth returns an Option that can be passed to New to configure the login credentials used by Chrome.
// Please do not check in real credentials to public repositories when using this in conjunction with GAIALogin.
func Auth(user, pass, gaiaID string) Option {
	return func(cfg *config) {
		cfg.user = user
		cfg.pass = pass
		cfg.gaiaID = gaiaID
	}
}

// Contact returns an Option that can be passed to New to configure the contact email used by Chrome for
// cross account challenge (go/ota-security). Please do not check in real credentials to public repositories
// when using this in conjunction with GAIALogin.
func Contact(contact string) Option {
	return func(cfg *config) {
		cfg.contact = contact
	}
}

// ParentAuth returns an Option that can be passed to New to configure the login credentials of a parent user.
// If the GAIA account specified by Auth is a supervised child user, this credential is used to go through the unicorn login flow.
// Please do not check in real credentials to public repositories when using this in conjunction with GAIALogin.
func ParentAuth(parentUser, parentPass string) Option {
	return func(cfg *config) {
		cfg.parentUser = parentUser
		cfg.parentPass = parentPass
	}
}

// KeepState returns an Option that can be passed to New to preserve the state such as
// files under /home/chronos and the user's existing cryptohome (if any) instead of
// wiping them before logging in.
func KeepState() Option {
	return func(cfg *config) { cfg.keepState = true }
}

// DeferLogin returns an option that instructs chrome.New to return before logging into a session.
// After successful return of chrome.New, you can call ContinueLogin to continue login.
func DeferLogin() Option {
	return func(cfg *config) { cfg.deferLogin = true }
}

// GAIALogin returns an Option that can be passed to New to perform a real GAIA-based login rather
// than the default fake login.
func GAIALogin() Option {
	return func(cfg *config) { cfg.loginMode = gaiaLogin }
}

// NoLogin returns an Option that can be passed to New to avoid logging in.
// Chrome is still restarted with testing-friendly behavior.
func NoLogin() Option {
	return func(cfg *config) { cfg.loginMode = noLogin }
}

// GuestLogin returns an Option that can be passed to New to log in as guest
// user.
func GuestLogin() Option {
	return func(cfg *config) {
		cfg.loginMode = guestLogin
		cfg.user = cryptohome.GuestUser
	}
}

// DontSkipOOBEAfterLogin returns an Option that can be passed to stay in OOBE after user login.
func DontSkipOOBEAfterLogin() Option {
	return func(cfg *config) {
		cfg.skipOOBEAfterLogin = false
	}
}

// Region returns an Option that can be passed to New to set the region deciding
// the locale used in the OOBE screen and the user sessions. region is a
// two-letter code such as "us", "fr", or "ja".
func Region(region string) Option {
	return func(cfg *config) {
		cfg.region = region
	}
}

// ProdPolicy returns an option that can be passed to New to let the device do a
// policy fetch upon login. By default, policies are not fetched.
// The default Device Management service is used.
func ProdPolicy() Option {
	return func(cfg *config) {
		cfg.policyEnabled = true
		cfg.dmsAddr = ""
	}
}

// DMSPolicy returns an option that can be passed to New to tell the device to fetch
// policies from the policy server at the given url. By default policies are not
// fetched.
func DMSPolicy(url string) Option {
	return func(cfg *config) {
		cfg.policyEnabled = true
		cfg.dmsAddr = url
	}
}

// EnterpriseEnroll returns an Option that can be passed to New to enable Enterprise
// Enrollment
func EnterpriseEnroll() Option {
	return func(cfg *config) { cfg.enroll = true }
}

// ARCDisabled returns an Option that can be passed to New to disable ARC.
func ARCDisabled() Option {
	return func(cfg *config) { cfg.arcMode = arcDisabled }
}

// ARCEnabled returns an Option that can be passed to New to enable ARC (without Play Store)
// for the user session with mock GAIA account.
func ARCEnabled() Option {
	return func(cfg *config) { cfg.arcMode = arcEnabled }
}

// ARCSupported returns an Option that can be passed to New to allow to enable ARC with Play Store gaia opt-in for the user
// session with real GAIA account.
// In this case ARC is not launched by default and is required to be launched by user policy or from UI.
func ARCSupported() Option {
	return func(cfg *config) { cfg.arcMode = arcSupported }
}

// RestrictARCCPU returns an Option that can be passed to New which controls whether
// to let Chrome use CGroups to limit the CPU time of ARC when in the background.
// Most ARC-related tests should not pass this option.
func RestrictARCCPU() Option {
	return func(cfg *config) { cfg.restrictARCCPU = true }
}

// CrashNormalMode tells the crash handling system to act like it would on a
// real device. If this option is not used, the Chrome instances created by this package
// will skip calling crash_reporter and write any dumps into /home/chronos/crash directly
// from breakpad. This option restores the normal behavior of calling crash_reporter.
func CrashNormalMode() Option {
	return func(cfg *config) { cfg.breakpadTestMode = false }
}

// ExtraArgs returns an Option that can be passed to New to append additional arguments to Chrome's command line.
func ExtraArgs(args ...string) Option {
	return func(cfg *config) { cfg.extraArgs = append(cfg.extraArgs, args...) }
}

// EnableFeatures returns an Option that can be passed to New to enable specific features in Chrome.
func EnableFeatures(features ...string) Option {
	return func(cfg *config) { cfg.enableFeatures = append(cfg.enableFeatures, features...) }
}

// DisableFeatures returns an Option that can be passed to New to disable specific features in Chrome.
func DisableFeatures(features ...string) Option {
	return func(cfg *config) { cfg.disableFeatures = append(cfg.disableFeatures, features...) }
}

// UnpackedExtension returns an Option that can be passed to New to make Chrome load an unpacked
// extension in the supplied directory.
// Ownership of the extension directory and its contents may be modified by New.
func UnpackedExtension(dir string) Option {
	return func(cfg *config) { cfg.extraExtDirs = append(cfg.extraExtDirs, dir) }
}

// LoadSigninProfileExtension loads the test extension which is allowed to run in the signin profile context.
// Private manifest key should be passed (see ui.SigninProfileExtension for details).
func LoadSigninProfileExtension(key string) Option {
	return func(cfg *config) { cfg.signinExtKey = key }
}
