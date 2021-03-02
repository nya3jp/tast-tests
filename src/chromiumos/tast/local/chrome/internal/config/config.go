// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package config defines a struct to hold configurations of chrome.Chrome.
package config

import (
	"encoding/json"
	"reflect"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/session"
)

const (
	// DefaultUser contains the email address used to log into Chrome when authentication credentials are not supplied.
	DefaultUser = "testuser@gmail.com"

	// DefaultPass contains the password we use to log into the DefaultUser account.
	DefaultPass   = "testpass"
	defaultGaiaID = "gaia-id"
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

// AuthType describes the type of authentication to be used in GAIA.
type AuthType string

// Valid values for AuthType.
const (
	UnknownAuth  AuthType = ""         // cannot determine the authentication type
	PasswordAuth AuthType = "password" // password based authentication
	ContactAuth  AuthType = "contact"  // contact email approval based authentication
)

// Config contains configurations for chrome.Chrome instance as requested by
// options to chrome.New.
//
// This is an immutable struct. Its fields must not be altered outside of Option
// and NewConfig.
//
// When TryReuseSession flag is set for a new chrome session, the configuration of the new session
// will be checked with the existing chrome session, to see if session reuse is possible.
// The "reuse_match" tag defines how a field should be handled when trying to reuse the existing
// chrome session. It has the following values:
// - "false": this field doesn't have to match for reused session
// - "true": this field have to match for reused session
// - "customized": Reuse checking logic is expected to be customized in customizedReuseCheck() function.
// This tag must be set for every field with one of the above values. Otherwise, unit test will fail.
type Config struct {
	User, Pass, GAIAID, Contact string    `reuse_match:"true"` // login credentials
	NormalizedUser              string    `reuse_match:"true"` // user with domain added, periods removed, etc.
	ParentUser, ParentPass      string    `reuse_match:"true"` // unicorn parent login credentials
	KeepState                   bool      `reuse_match:"false"`
	DeferLogin                  bool      `reuse_match:"customized"`
	LoginMode                   LoginMode `reuse_match:"customized"`
	TryReuseSession             bool      `reuse_match:"false"` // try to reuse existing login session if configuration matches
	EnableLoginVerboseLogs      bool      `reuse_match:"true"`  // enable verbose logging in some login related files
	VKEnabled                   bool      `reuse_match:"true"`
	SkipOOBEAfterLogin          bool      `reuse_match:"false"` // skip OOBE post user login
	InstallWebApp               bool      `reuse_match:"true"`  // auto install essential apps after user login
	Region                      string    `reuse_match:"true"`
	PolicyEnabled               bool      `reuse_match:"true"` // flag to enable policy fetch
	DMSAddr                     string    `reuse_match:"true"` // Device Management URL, or empty if using default
	Enroll                      bool      `reuse_match:"true"` // whether device should be enrolled
	ARCMode                     ARCMode   `reuse_match:"true"`
	RestrictARCCPU              bool      `reuse_match:"true"`  // a flag to control cpu restrictions on ARC
	EnableRestoreTabs           bool      `reuse_match:"false"` // Skip creating browser windws on login

	// If BreakpadTestMode is true, tell Chrome's breakpad to always write
	// dumps directly to a hardcoded directory.
	BreakpadTestMode bool     `reuse_match:"true"`
	ExtraArgs        []string `reuse_match:"true"`
	LacrosExtraArgs  []string `reuse_match:"true"`
	EnableFeatures   []string `reuse_match:"true"`
	DisableFeatures  []string `reuse_match:"true"`

	// reuse_match of extensions will be handled in Chrome.New().
	ExtraExtDirs []string `reuse_match:"customized"` // directories containing all extra unpacked extensions to load
	SigninExtKey string   `reuse_match:"customized"` // private key for signin profile test extension manifest
}

// Option is a self-referential function can be used to configure Chrome.
// See https://commandcenter.blogspot.com.au/2014/01/self-referential-functions-and-design.html
// for details about this pattern.
type Option func(cfg *Config) error

// NewConfig constructs Config from a list of options given to chrome.New.
func NewConfig(opts []Option) (*Config, error) {
	cfg := &Config{
		User:                   DefaultUser,
		Pass:                   DefaultPass,
		GAIAID:                 defaultGaiaID,
		KeepState:              false,
		LoginMode:              FakeLogin,
		VKEnabled:              false,
		SkipOOBEAfterLogin:     true,
		EnableLoginVerboseLogs: false,
		InstallWebApp:          false,
		Region:                 "us",
		PolicyEnabled:          false,
		Enroll:                 false,
		BreakpadTestMode:       true,
		EnableRestoreTabs:      false,
	}
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}

	// TODO(rrsilva, crbug.com/1109176) - Disable login-related verbose logging
	// in all tests once the issue is solved.
	cfg.EnableLoginVerboseLogs = true

	// This works around https://crbug.com/358427.
	if cfg.LoginMode == GAIALogin {
		var err error
		if cfg.NormalizedUser, err = session.NormalizeEmail(cfg.User, true); err != nil {
			return nil, errors.Wrapf(err, "failed to normalize email %q", cfg.User)
		}
	} else {
		cfg.NormalizedUser = cfg.User
	}

	return cfg, nil
}

// Marshal marshals the Config struct to bytes.
func (c *Config) Marshal() ([]byte, error) {
	return json.Marshal(c)
}

// Unmarshal unmarshals the data to a Config struct.
func Unmarshal(data []byte) (*Config, error) {
	cfg := &Config{}
	if err := json.Unmarshal(data, cfg); err != nil {
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
	t := reflect.TypeOf(c).Elem()
	val1 := reflect.ValueOf(c).Elem()
	val2 := reflect.ValueOf(newCfg).Elem()
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
	if newCfg.DeferLogin {
		return errors.New("session with DeferLogin cannot be reused")
	}
	if newCfg.LoginMode == NoLogin {
		return errors.New("session with NoLogin as LoginMode cannot be reused")
	}
	if newCfg.LoginMode != c.LoginMode {
		return errors.Errorf("LoginMode has different values and cannot be reused: %v vs. %v", c.LoginMode, newCfg.LoginMode)
	}

	return nil
}
