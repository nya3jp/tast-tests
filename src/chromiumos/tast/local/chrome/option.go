// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"math/rand"
	"strings"
	"time"

	"chromiumos/tast/errors"
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
	return func(cfg *config.Config) error {
		cfg.InstallWebApp = true
		return nil
	}
}

// EnableLoginVerboseLogs returns an Option that enables verbose logging for some login-related files.
func EnableLoginVerboseLogs() Option {
	return func(cfg *config.Config) error {
		cfg.EnableLoginVerboseLogs = true
		return nil
	}
}

// VKEnabled returns an Option that force enable virtual keyboard.
// VKEnabled option appends "--enable-virtual-keyboard" to chrome initialization and also checks VK connection after user login.
// Note: This option can not be used by ARC tests as some boards block VK background from presence.
func VKEnabled() Option {
	return func(cfg *config.Config) error {
		cfg.VKEnabled = true
		return nil
	}
}

// Auth returns an Option that can be passed to New to configure the login credentials used by Chrome.
// Please do not check in real credentials to public repositories when using this in conjunction with GAIALogin.
func Auth(user, pass, gaiaID string) Option {
	return func(cfg *config.Config) error {
		cfg.User = user
		cfg.Pass = pass
		cfg.GAIAID = gaiaID
		return nil
	}
}

var random = rand.New(rand.NewSource(time.Now().UnixNano()))

// AuthPool returns an Option that can be passed to New to configure the login
// credentials used by Chrome.
//
// creds is a string containing multiple credentials separated by newlines:
//  user1:pass1
//  user2:pass2
//  user3:pass3
//  ...
//
// This option randomly picks one user/pass pair. A chosen user is written to
// logs in chrome.New, as well as available via Chrome.User.
func AuthPool(creds string) Option {
	return func(cfg *config.Config) error {
		cs, err := parseCreds(creds)
		if err != nil {
			return err
		}
		c := cs[random.Intn(len(cs))]
		cfg.User = c.user
		cfg.Pass = c.pass
		return nil
	}
}

type cred struct {
	user, pass string
}

func parseCreds(creds string) ([]cred, error) {
	// Note: Do not include creds in error messages to avoid accidental
	// credential leaks in logs.
	var cs []cred
	for i, line := range strings.Split(creds, "\n") {
		line = strings.TrimSpace(line)
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		ps := strings.SplitN(line, ":", 2)
		if len(ps) != 2 {
			return nil, errors.Errorf("failed to parse credential list: line %d: does not contain a colon", i+1)
		}
		cs = append(cs, cred{
			user: ps[0],
			pass: ps[1],
		})
	}
	return cs, nil
}

// Contact returns an Option that can be passed to New to configure the contact email used by Chrome for
// cross account challenge (go/ota-security). Please do not check in real credentials to public repositories
// when using this in conjunction with GAIALogin.
func Contact(contact string) Option {
	return func(cfg *config.Config) error {
		cfg.Contact = contact
		return nil
	}
}

// ParentAuth returns an Option that can be passed to New to configure the login credentials of a parent user.
// If the GAIA account specified by Auth is a supervised child user, this credential is used to go through the unicorn login flow.
// Please do not check in real credentials to public repositories when using this in conjunction with GAIALogin.
func ParentAuth(parentUser, parentPass string) Option {
	return func(cfg *config.Config) error {
		cfg.ParentUser = parentUser
		cfg.ParentPass = parentPass
		return nil
	}
}

// KeepState returns an Option that can be passed to New to preserve the state such as
// files under /home/chronos and the user's existing cryptohome (if any) instead of
// wiping them before logging in.
func KeepState() Option {
	return func(cfg *config.Config) error {
		cfg.KeepState = true
		return nil
	}
}

// DeferLogin returns an option that instructs chrome.New to return before logging into a session.
// After successful return of chrome.New, you can call ContinueLogin to continue login.
func DeferLogin() Option {
	return func(cfg *config.Config) error {
		cfg.DeferLogin = true
		return nil
	}
}

// GAIALogin returns an Option that can be passed to New to perform a real GAIA-based login rather
// than the default fake login.
func GAIALogin() Option {
	return func(cfg *config.Config) error {
		cfg.LoginMode = config.GAIALogin
		return nil
	}
}

// NoLogin returns an Option that can be passed to New to avoid logging in.
// Chrome is still restarted with testing-friendly behavior.
func NoLogin() Option {
	return func(cfg *config.Config) error {
		cfg.LoginMode = config.NoLogin
		return nil
	}
}

// GuestLogin returns an Option that can be passed to New to log in as guest
// user.
func GuestLogin() Option {
	return func(cfg *config.Config) error {
		cfg.LoginMode = config.GuestLogin
		cfg.User = cryptohome.GuestUser
		return nil
	}
}

// DontSkipOOBEAfterLogin returns an Option that can be passed to stay in OOBE after user login.
func DontSkipOOBEAfterLogin() Option {
	return func(cfg *config.Config) error {
		cfg.SkipOOBEAfterLogin = false
		return nil
	}
}

// Region returns an Option that can be passed to New to set the region deciding
// the locale used in the OOBE screen and the user sessions. region is a
// two-letter code such as "us", "fr", or "ja".
func Region(region string) Option {
	return func(cfg *config.Config) error {
		cfg.Region = region
		return nil
	}
}

// ProdPolicy returns an option that can be passed to New to let the device do a
// policy fetch upon login. By default, policies are not fetched.
// The default Device Management service is used.
func ProdPolicy() Option {
	return func(cfg *config.Config) error {
		cfg.PolicyEnabled = true
		cfg.DMSAddr = ""
		return nil
	}
}

// DMSPolicy returns an option that can be passed to New to tell the device to fetch
// policies from the policy server at the given url. By default policies are not
// fetched.
func DMSPolicy(url string) Option {
	return func(cfg *config.Config) error {
		cfg.PolicyEnabled = true
		cfg.DMSAddr = url
		return nil
	}
}

// EnterpriseEnroll returns an Option that can be passed to New to enable Enterprise
// Enrollment
func EnterpriseEnroll() Option {
	return func(cfg *config.Config) error {
		cfg.Enroll = true
		return nil
	}
}

// ARCDisabled returns an Option that can be passed to New to disable ARC.
func ARCDisabled() Option {
	return func(cfg *config.Config) error {
		cfg.ARCMode = config.ARCDisabled
		return nil
	}
}

// ARCEnabled returns an Option that can be passed to New to enable ARC (without Play Store)
// for the user session with mock GAIA account.
func ARCEnabled() Option {
	return func(cfg *config.Config) error {
		cfg.ARCMode = config.ARCEnabled
		return nil
	}
}

// ARCSupported returns an Option that can be passed to New to allow to enable ARC with Play Store gaia opt-in for the user
// session with real GAIA account.
// In this case ARC is not launched by default and is required to be launched by user policy or from UI.
func ARCSupported() Option {
	return func(cfg *config.Config) error {
		cfg.ARCMode = config.ARCSupported
		return nil
	}
}

// RestrictARCCPU returns an Option that can be passed to New which controls whether
// to let Chrome use CGroups to limit the CPU time of ARC when in the background.
// Most ARC-related tests should not pass this option.
func RestrictARCCPU() Option {
	return func(cfg *config.Config) error {
		cfg.RestrictARCCPU = true
		return nil
	}
}

// EnableRestoreTabs returns an Option that can be passed to New which controls whether
// to let Chrome use CGroups to limit the CPU time of ARC when in the background.
// Most ARC-related tests should not pass this option.
func EnableRestoreTabs() Option {
	return func(cfg *config.Config) error {
		cfg.EnableRestoreTabs = true
		return nil
	}
}

// CrashNormalMode tells the crash handling system to act like it would on a
// real device. If this option is not used, the Chrome instances created by this package
// will skip calling crash_reporter and write any dumps into /home/chronos/crash directly
// from breakpad. This option restores the normal behavior of calling crash_reporter.
func CrashNormalMode() Option {
	return func(cfg *config.Config) error {
		cfg.BreakpadTestMode = false
		return nil
	}
}

// ExtraArgs returns an Option that can be passed to New to append additional arguments to Chrome's command line.
func ExtraArgs(args ...string) Option {
	return func(cfg *config.Config) error {
		cfg.ExtraArgs = append(cfg.ExtraArgs, args...)
		return nil
	}
}

// LacrosExtraArgs returns an Option that can be passed to New to append additional arguments to Lacros Chrome's command line.
func LacrosExtraArgs(args ...string) Option {
	return func(cfg *config.Config) error {
		cfg.LacrosExtraArgs = append(cfg.LacrosExtraArgs, args...)
		return nil
	}
}

// EnableFeatures returns an Option that can be passed to New to enable specific features in Chrome.
func EnableFeatures(features ...string) Option {
	return func(cfg *config.Config) error {
		cfg.EnableFeatures = append(cfg.EnableFeatures, features...)
		return nil
	}
}

// DisableFeatures returns an Option that can be passed to New to disable specific features in Chrome.
func DisableFeatures(features ...string) Option {
	return func(cfg *config.Config) error {
		cfg.DisableFeatures = append(cfg.DisableFeatures, features...)
		return nil
	}
}

// UnpackedExtension returns an Option that can be passed to New to make Chrome load an unpacked
// extension in the supplied directory.
// The specified directory is copied to a different location before loading, so modifications to
// the directory do not take effect after starting Chrome.
func UnpackedExtension(dir string) Option {
	return func(cfg *config.Config) error {
		cfg.ExtraExtDirs = append(cfg.ExtraExtDirs, dir)
		return nil
	}
}

// LoadSigninProfileExtension loads the test extension which is allowed to run in the signin profile context.
// Private manifest key should be passed (see ui.SigninProfileExtension for details).
func LoadSigninProfileExtension(key string) Option {
	return func(cfg *config.Config) error {
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
	return func(cfg *config.Config) error {
		cfg.TryReuseSession = true
		return nil
	}
}
