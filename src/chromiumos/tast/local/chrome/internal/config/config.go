// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package config defines a struct to hold configurations of chrome.Chrome.
package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/session"
)

const (
	// DefaultUser contains the email address used to log into Chrome when authentication credentials are not supplied.
	DefaultUser = "testuser@gmail.com"

	// DefaultPass contains the password we use to log into the DefaultUser account.
	DefaultPass = "testpass"
)

// ARCMode describes the mode that ARC should be put into.
type ARCMode int

// Valid values for ARCMode.
const (
	ARCDisabled ARCMode = iota
	ARCEnabled
	ARCSupported // ARC is supported and can be launched by user policy
)

// LoginMode describes the user mode for the login.
type LoginMode int

// Valid values for LoginMode.
const (
	NoLogin    LoginMode = iota // restart Chrome but don't log in
	FakeLogin                   // fake login with no authentication
	GAIALogin                   // real network-based login using GAIA backend
	GuestLogin                  // sign in as ephemeral guest user
)

// EnrollMode describes how the test should enroll.
type EnrollMode int

// Valid values for EnrollMode.
const (
	NoEnroll      EnrollMode = iota // do not enroll device
	FakeEnroll                      // enroll with a fake, local device management server
	GAIAEnroll                      // real network based enrollment using a real, live device management server
	GAIAZTEEnroll                   // real network based ZTE enrollment using a real, live device management server
)

// AuthType describes the type of authentication to be used in GAIA.
type AuthType string

// Valid values for AuthType.
const (
	UnknownAuth  AuthType = ""         // cannot determine the authentication type
	PasswordAuth AuthType = "password" // password based authentication
	ContactAuth  AuthType = "contact"  // contact email approval based authentication
)

// Creds contains credentials to log into a Chrome user session.
type Creds struct {
	// User is the user name of a user account. It is typically an email
	// address (e.g. example@gmail.com).
	User string
	// Pass is the password of a user account.
	Pass string

	// GAIAID is a GAIA ID used on fake logins. If it is empty, an ID is
	// generated from the user name. The field is ignored on other type of
	// logins.
	GAIAID string

	// Contact is an email address of a user who owns a test account.
	// When logging in with a test account, its contact user may be
	// notified of a login attempt and asked for approval.
	Contact string

	// ParentUser is the user name of a parent account. It is used to
	// approve a login attempt when a child account is supervised by a
	// parent account.
	ParentUser string
	// ParentPass is the pass of a parent account. It is used to approve
	// a login attempt when a child account is supervised by a parent
	// account.
	ParentPass string
}

// defaultCreds is the default credentials used for fake logins.
var defaultCreds = Creds{
	User: DefaultUser,
	Pass: DefaultPass,
}

// ParseCreds parses a string containing a list of credentials.
//
// creds is a string containing multiple credentials separated by newlines:
//
//	user1:pass1
//	user2:pass2
//	user3:pass3
//	...
func ParseCreds(creds string) ([]Creds, error) {
	// Note: Do not include creds in error messages to avoid accidental
	// credential leaks in logs.
	var cs []Creds
	for i, line := range strings.Split(creds, "\n") {
		line = strings.TrimSpace(line)
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		ps := strings.SplitN(line, ":", 2)
		if len(ps) != 2 {
			return nil, errors.Errorf("failed to parse credential list: line %d: does not contain a colon", i+1)
		}
		cs = append(cs, Creds{
			User: ps[0],
			Pass: ps[1],
		})
	}
	return cs, nil
}

// Config contains configurations for chrome.Chrome instance as requested by
// options to chrome.New.
//
// This is an immutable struct. Modification outside NewConfig is prohibited.
type Config struct {
	m MutableConfig
}

// Creds returns login credentials.
func (c *Config) Creds() Creds { return c.m.Creds }

// NormalizedUser returns a normalized user email.
func (c *Config) NormalizedUser() string { return c.m.NormalizedUser }

// KeepState returns whether to keep existing user profiles.
func (c *Config) KeepState() bool { return c.m.KeepState }

// KeepOwnership returns whether to keep existing ownership of the device.
// This is critical for enrolled devices.
func (c *Config) KeepOwnership() bool { return c.m.KeepOwnership }

// DeferLogin returns whether to defer login in chrome.New. If it is true,
// users should call Chrome.ContinueLogin to continue login.
func (c *Config) DeferLogin() bool { return c.m.DeferLogin }

// LoginMode returns a login mode.
func (c *Config) LoginMode() LoginMode { return c.m.LoginMode }

// TryReuseSession returns whether to try reusing a current user session.
func (c *Config) TryReuseSession() bool { return c.m.TryReuseSession }

// EnableLoginVerboseLogs returns whether to enable verbose logs on login.
func (c *Config) EnableLoginVerboseLogs() bool { return c.m.EnableLoginVerboseLogs }

// VKEnabled returns whether to force enable the virtual keyboard.
func (c *Config) VKEnabled() bool { return c.m.VKEnabled }

// SkipOOBEAfterLogin returns whether to skip OOBE after login.
func (c *Config) SkipOOBEAfterLogin() bool { return c.m.SkipOOBEAfterLogin }

// WaitForCryptohome returns whether to wait for the cryptohome mount after login.
func (c *Config) WaitForCryptohome() bool { return c.m.WaitForCryptohome }

// CustomLoginTimeout returns a custom timeout for login. If 0, use chrome.LoginTimeout.
func (c *Config) CustomLoginTimeout() time.Duration {
	return time.Duration(c.m.CustomLoginTimeout) * time.Nanosecond
}

// InstallWebApp returns whether to automatically install essential web apps.
func (c *Config) InstallWebApp() bool { return c.m.InstallWebApp }

// Region returns a region of a user session.
func (c *Config) Region() string { return c.m.Region }

// PolicyEnabled returns whether to enable policy.
func (c *Config) PolicyEnabled() bool { return c.m.PolicyEnabled }

// DMSAddr returns the address of a device management server.
func (c *Config) DMSAddr() string { return c.m.DMSAddr }

// RealtimeReportingAddr returns the address of a realtime reporting endpoint.
func (c *Config) RealtimeReportingAddr() string { return c.m.RealtimeReportingAddr }

// EncryptedReportingAddr returns the address of a encrypted reporting endpoint.
func (c *Config) EncryptedReportingAddr() string { return c.m.EncryptedReportingAddr }

// EnrollMode returns an enterprise enrollment mode.
func (c *Config) EnrollMode() EnrollMode { return c.m.EnrollMode }

// EnrollmentCreds returns the credential used to enroll the device.
func (c *Config) EnrollmentCreds() Creds { return c.m.EnrollmentCreds }

// DisablePolicyKeyVerification returns whether to disable policy key verification in Chrome.
func (c *Config) DisablePolicyKeyVerification() bool { return c.m.DisablePolicyKeyVerification }

// ARCMode returns the mode of ARC.
func (c *Config) ARCMode() ARCMode { return c.m.ARCMode }

// ARCUseHugePages returns the memory mode of the guest memory.
func (c *Config) ARCUseHugePages() bool { return c.m.ARCUseHugePages }

// UnRestrictARCCPU returns whether to not restrict CPU usage of ARC in background.
func (c *Config) UnRestrictARCCPU() bool { return c.m.UnRestrictARCCPU }

// BreakpadTestMode returns whether to tell Chrome's breakpad to always write
// dumps directly to a hard-coded directory.
func (c *Config) BreakpadTestMode() bool { return c.m.BreakpadTestMode }

// ExtraArgs returns extra arguments to pass to Chrome.
func (c *Config) ExtraArgs() []string { return append([]string(nil), c.m.ExtraArgs...) }

// LacrosExtraArgs returns extra arguments to pass to Lacros Chrome.
func (c *Config) LacrosExtraArgs() []string { return append([]string(nil), c.m.LacrosExtraArgs...) }

// EnableFeatures returns extra Chrome features to enable.
func (c *Config) EnableFeatures() []string { return append([]string(nil), c.m.EnableFeatures...) }

// LacrosEnableFeatures returns extra Lacros Chrome features to enable.
func (c *Config) LacrosEnableFeatures() []string {
	return append([]string(nil), c.m.LacrosEnableFeatures...)
}

// DisableFeatures returns extra Chrome features to disable.
func (c *Config) DisableFeatures() []string { return append([]string(nil), c.m.DisableFeatures...) }

// LacrosDisableFeatures returns extra Lacros Chrome features to disable.
func (c *Config) LacrosDisableFeatures() []string {
	return append([]string(nil), c.m.LacrosDisableFeatures...)
}

// ExtraExtDirs returns directories containing extra unpacked extensions to load.
func (c *Config) ExtraExtDirs() []string { return append([]string(nil), c.m.ExtraExtDirs...) }

// LacrosExtraExtDirs returns directories containing extra Lacros unpacked extensions to load.
func (c *Config) LacrosExtraExtDirs() []string {
	return append([]string(nil), c.m.LacrosExtraExtDirs...)
}

// SigninExtKey returns a private key for the sign-in profile test extension.
func (c *Config) SigninExtKey() string { return c.m.SigninExtKey }

// EnableRestoreTabs returns true if creating browser windows on login should be skipped.
func (c *Config) EnableRestoreTabs() bool { return c.m.EnableRestoreTabs }

// SkipForceOnlineSignInForTesting returns true if online sign-in enforcement should be disabled.
func (c *Config) SkipForceOnlineSignInForTesting() bool { return c.m.SkipForceOnlineSignInForTesting }

// RemoveNotification returns true if to remove all notiviations after restarting.
func (c *Config) RemoveNotification() bool { return c.m.RemoveNotification }

// HideCrashRestoreBubble returns true if to skip "Chrome did not shut down correctly" bubble.
func (c *Config) HideCrashRestoreBubble() bool { return c.m.HideCrashRestoreBubble }

// ForceLaunchBrowser returns true if to force FullRestoreService to launch browser for telemetry tests.
func (c *Config) ForceLaunchBrowser() bool { return c.m.ForceLaunchBrowser }

// EphemeralUser returns true if user mount should be validated to be ephemeral, e.g. for guest user.
func (c *Config) EphemeralUser() bool { return c.m.EphemeralUser }

// EnablePersonalizationHub returns true if the Personalization Hub is enabled.
func (c *Config) EnablePersonalizationHub() bool { return c.m.EnablePersonalizationHub }

// UseSandboxGaia returns true if the sandbox instance of Gaia should be used.
func (c *Config) UseSandboxGaia() bool { return c.m.UseSandboxGaia }

// TestExtOAuthClientID returns the OAuth Client ID to use in the test extension
// if one was provided.
func (c *Config) TestExtOAuthClientID() string { return c.m.TestExtOAuthClientID }

// EnableHIDScreenOnOOBE returns true if to keep the default behavior of OOBE and not force skip HID
// detection screen in OOBE.
func (c *Config) EnableHIDScreenOnOOBE() bool { return c.m.EnableHIDScreenOnOOBE }

// MutableConfig is a mutable version of Config. MutableConfig is wrapped with
// Config to prevent mutation after it is returned by NewConfig.
//
// When TryReuseSession flag is set for a new chrome session, the configuration of the new session
// will be checked with the existing chrome session, to see if session reuse is possible.
// The "reuse_match" tag defines how a field should be handled when trying to reuse the existing
// chrome session. It has the following values:
// - "false": this field doesn't have to match for reused session
// - "true": this field have to match for reused session
// - "customized": Reuse checking logic is expected to be customized in customizedReuseCheck() function.
// This tag must be set for every field with one of the above values. Otherwise, unit test will fail.
type MutableConfig struct {
	Creds                           Creds      `reuse_match:"true"`
	NormalizedUser                  string     `reuse_match:"true"`
	KeepState                       bool       `reuse_match:"false"`
	KeepOwnership                   bool       `reuse_match:"true"`
	DeferLogin                      bool       `reuse_match:"customized"`
	EnableRestoreTabs               bool       `reuse_match:"false"`
	LoginMode                       LoginMode  `reuse_match:"customized"`
	TryReuseSession                 bool       `reuse_match:"false"`
	EnableLoginVerboseLogs          bool       `reuse_match:"true"`
	VKEnabled                       bool       `reuse_match:"true"`
	SkipOOBEAfterLogin              bool       `reuse_match:"false"`
	WaitForCryptohome               bool       `reuse_match:"false"`
	CustomLoginTimeout              int64      `reuse_match:"false"` // time.Duration can not be serialized to JSON. Store duration in nanoseconds.
	InstallWebApp                   bool       `reuse_match:"true"`
	Region                          string     `reuse_match:"true"`
	PolicyEnabled                   bool       `reuse_match:"true"`
	DMSAddr                         string     `reuse_match:"true"`
	RealtimeReportingAddr           string     `reuse_match:"true"`
	EncryptedReportingAddr          string     `reuse_match:"true"`
	EnrollMode                      EnrollMode `reuse_match:"true"`
	EnrollmentCreds                 Creds      `reuse_match:"true"`
	DisablePolicyKeyVerification    bool       `reuse_match:"true"`
	ARCMode                         ARCMode    `reuse_match:"true"`
	ARCUseHugePages                 bool       `reuse_match:"true"`
	UnRestrictARCCPU                bool       `reuse_match:"true"`
	BreakpadTestMode                bool       `reuse_match:"true"`
	ExtraArgs                       []string   `reuse_match:"true"`
	LacrosExtraArgs                 []string   `reuse_match:"true"`
	EnableFeatures                  []string   `reuse_match:"true"`
	LacrosEnableFeatures            []string   `reuse_match:"true"`
	DisableFeatures                 []string   `reuse_match:"true"`
	LacrosDisableFeatures           []string   `reuse_match:"true"`
	ExtraExtDirs                    []string   `reuse_match:"customized"`
	LacrosExtraExtDirs              []string   `reuse_match:"customized"`
	SigninExtKey                    string     `reuse_match:"customized"`
	SkipForceOnlineSignInForTesting bool       `reuse_match:"true"`
	RemoveNotification              bool       `reuse_match:"true"`
	HideCrashRestoreBubble          bool       `reuse_match:"true"`
	ForceLaunchBrowser              bool       `reuse_match:"true"`
	EphemeralUser                   bool       `reuse_match:"true"`
	EnablePersonalizationHub        bool       `reuse_match:"true"`
	UseSandboxGaia                  bool       `reuse_match:"true"`
	TestExtOAuthClientID            string     `reuse_match:"true"`
	EnableHIDScreenOnOOBE           bool       `reuse_match:"true"`
}

// Option is a self-referential function can be used to configure Chrome.
// See https://commandcenter.blogspot.com.au/2014/01/self-referential-functions-and-design.html
// for details about this pattern.
type Option func(cfg *MutableConfig) error

// NewConfig constructs Config from a list of options given to chrome.New.
func NewConfig(opts []Option) (*Config, error) {
	cfg := &Config{
		m: MutableConfig{
			Creds:                           defaultCreds,
			KeepState:                       false,
			KeepOwnership:                   false,
			LoginMode:                       FakeLogin,
			VKEnabled:                       false,
			SkipOOBEAfterLogin:              true,
			WaitForCryptohome:               true,
			CustomLoginTimeout:              0,
			EnableLoginVerboseLogs:          false,
			InstallWebApp:                   false,
			Region:                          "us",
			PolicyEnabled:                   false,
			EnrollMode:                      NoEnroll,
			EnrollmentCreds:                 Creds{},
			DisablePolicyKeyVerification:    false,
			BreakpadTestMode:                true,
			EnableRestoreTabs:               false,
			SkipForceOnlineSignInForTesting: false,
			RemoveNotification:              true,
			HideCrashRestoreBubble:          false,
			ForceLaunchBrowser:              false,
			EphemeralUser:                   false,
			EnablePersonalizationHub:        true,
			UseSandboxGaia:                  false,
			EnableHIDScreenOnOOBE:           false,
		},
	}
	for _, opt := range opts {
		if err := opt(&cfg.m); err != nil {
			return nil, err
		}
	}

	// TODO(rrsilva, crbug.com/1109176) - Disable login-related verbose logging
	// in all tests once the issue is solved.
	cfg.m.EnableLoginVerboseLogs = true

	// This works around https://crbug.com/358427.
	var err error
	if cfg.m.NormalizedUser, err = session.NormalizeEmail(cfg.m.Creds.User, true); err != nil {
		return nil, errors.Wrapf(err, "failed to normalize email %q", cfg.m.Creds.User)
	}

	// Logging in with a fake account requires a non-empty unique GAIA ID.
	// Generate one when it's empty.
	if cfg.m.LoginMode == FakeLogin && cfg.m.Creds.GAIAID == "" {
		h := sha256.Sum256([]byte(cfg.m.NormalizedUser))
		cfg.m.Creds.GAIAID = hex.EncodeToString(h[:])
	}

	return cfg, nil
}

// Marshal marshals the Config struct to bytes.
func (c *Config) Marshal() ([]byte, error) {
	return json.Marshal(&c.m)
}

// Unmarshal unmarshals the data to a Config struct.
func Unmarshal(data []byte) (*Config, error) {
	cfg := &Config{}
	if err := json.Unmarshal(data, &cfg.m); err != nil {
		return nil, err
	}
	return cfg, nil
}

// VerifySessionReuse compares two configurations to see if they are compatible for the existing
// Chrome session to be re-used. This function is called when TryReuseSession option is set for
// the new Chrome session request.
// Default comparison logic is implemented here. Customized comparison logic goes to
// customizedReuseCheck() function .
func (c *Config) VerifySessionReuse(newCfg *Config) error {
	t := reflect.TypeOf(&c.m).Elem()
	val1 := reflect.ValueOf(&c.m).Elem()
	val2 := reflect.ValueOf(&newCfg.m).Elem()
	// Iterate over all available fields and compare fields requiring exact match.
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Get the field tag.
		reuseMatch := field.Tag.Get("reuse_match")
		if reuseMatch == "false" || reuseMatch == "customized" {
			continue
		}
		if reuseMatch != "true" {
			// Not a known "reuse_match" value. Deny the reuse.
			return errors.Errorf("reuse_match tag for field %q has unexpected value %q", field.Name, reuseMatch)
		}

		fv1 := val1.Field(i)
		fv2 := val2.Field(i)
		if !reflect.DeepEqual(fv1.Interface(), fv2.Interface()) {
			return errors.Errorf("field %q has different values and cannot be reused", field.Name)
		}
	}

	// Check fields requiring customized comparison logic.
	return c.customizedReuseCheck(newCfg)
}

// customizedReuseCheck provides customized session reuse checking logic for fields with
// `reuse_match:"customized"` tag. If a newly defined Config field needs to be handled explicitly
// for session reuse, the customized logic should be added in this function.
func (c *Config) customizedReuseCheck(newCfg *Config) error {
	// For both DeferLogin and NoLogin, the Chrome UI will stay at the OOBE page and no login should
	// be performed. The current session will not be reused because:
	// - If the current session has already logged in, UI restart is required to return to OOBE.
	// - If the current session happens to be at the OOBE page, test API extension is not accessible
	// yet and we are not sure if the session can be reused.
	if newCfg.DeferLogin() {
		return errors.New("session with DeferLogin cannot be reused")
	}
	if newCfg.LoginMode() == NoLogin {
		return errors.New("session with NoLogin as LoginMode cannot be reused")
	}
	if newCfg.LoginMode() != c.LoginMode() {
		return errors.Errorf("LoginMode has different values and cannot be reused: %v vs. %v", c.LoginMode(), newCfg.LoginMode())
	}

	return nil
}
