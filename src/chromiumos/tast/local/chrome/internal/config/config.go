// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package config defines a struct to hold configurations of chrome.Chrome.
package config

import (
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
// The "reuse_match" tag defines whether this field needs to match when trying to
// reuse the existing chrome session (i.e., when TryReuseSession is true). The tag has
// the following values:
// - "true": the field must match for reused session
// - "explicit": these fields will be handled explicitly in IsSessionReusable().
// - other value: the field doesn't have to match
type Config struct {
	User, Pass, GAIAID, Contact string `reuse_match:"true"` // login credentials
	NormalizedUser              string // user with domain added, periods removed, etc.
	ParentUser, ParentPass      string // unicorn parent login credentials
	KeepState                   bool
	DeferLogin                  bool      `reuse_match:"explict"`
	LoginMode                   LoginMode `reuse_match:"explict"`
	TryReuseSession             bool      // try to reuse exiting login session if configuration matches
	EnableLoginVerboseLogs      bool      `reuse_match:"true"` // enable verbose logging in some login related files
	VKEnabled                   bool      `reuse_match:"true"`
	SkipOOBEAfterLogin          bool      // skip OOBE post user login
	InstallWebApp               bool      `reuse_match:"true"` // auto install essential apps after user login
	Region                      string    `reuse_match:"true"`
	PolicyEnabled               bool      `reuse_match:"true"` // flag to enable policy fetch
	DMSAddr                     string    `reuse_match:"true"` // Device Management URL, or empty if using default
	Enroll                      bool      `reuse_match:"true"` // whether device should be enrolled
	ARCMode                     ARCMode   `reuse_match:"true"`
	RestrictARCCPU              bool      `reuse_match:"true"` // a flag to control cpu restrictions on ARC

	// If BreakpadTestMode is true, tell Chrome's breakpad to always write
	// dumps directly to a hardcoded directory.
	BreakpadTestMode bool     `reuse_match:"true"`
	ExtraArgs        []string `reuse_match:"explict"`
	EnableFeatures   []string `reuse_match:"explict"`
	DisableFeatures  []string `reuse_match:"explict"`

	// reuse_match of extensions will be handled in Chrome.New().
	ExtraExtDirs []string `reuse_match:"explict"` // directories containing all extra unpacked extensions to load
	SigninExtKey string   `reuse_match:"explict"` // private key for signin profile test extension manifest
}

// Option is a self-referential function can be used to configure Chrome.
// See https://commandcenter.blogspot.com.au/2014/01/self-referential-functions-and-design.html
// for details about this pattern.
type Option func(cfg *Config)

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
	}
	for _, opt := range opts {
		opt(cfg)
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

// IsSessionReusable compares two configurations to see if they are compatible for the existing
// Chrome session to be re-used. This function is called when TryReuseSession option is set for
// the new Chrome session request.
//
// If a newly defined Config field needs to be handled explictly for session reuse, the comparison
// logic should be added in this function.
func (c *Config) IsSessionReusable(new *Config) bool {
	t := reflect.TypeOf(c).Elem()
	val1 := reflect.ValueOf(c).Elem()
	val2 := reflect.ValueOf(new).Elem()
	// Iterate over all available fields and compare field with reuse_match:"true" tag.
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Get the field tag.
		reuseMatch := field.Tag.Get("reuse_match")
		if reuseMatch != "true" {
			continue
		}

		fv1 := val1.Field(i)
		fv2 := val2.Field(i)
		if !reflect.DeepEqual(fv1.Interface(), fv2.Interface()) {
			return false
		}
	}

	// Explicitly process other config fields.
	if new.DeferLogin || new.LoginMode == NoLogin {
		return false
	}
	if new.LoginMode != c.LoginMode {
		return false
	}
	if !match(c.ExtraArgs, new.ExtraArgs) {
		return false
	}
	if !match(c.EnableFeatures, new.EnableFeatures) {
		return false
	}
	if !match(c.DisableFeatures, new.DisableFeatures) {
		return false
	}
	return true
}

// match returns true if slice1 and slice2 contains same elements.
func match(slc1, slc2 []string) bool {
	if len(slc1) != len(slc2) {
		return false
	}
	comp := make(map[string]int, len(slc1))
	for _, e := range slc1 {
		comp[e]++
	}
	for _, e := range slc2 {
		if _, ok := comp[e]; !ok {
			return false
		}
		comp[e]--
		if comp[e] == 0 {
			delete(comp, e)
		}
	}
	if len(comp) != 0 {
		return false
	}
	return true
}
