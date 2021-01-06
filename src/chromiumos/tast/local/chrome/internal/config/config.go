// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package config defines a struct to hold configurations of chrome.Chrome.
package config

import (
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
type Config struct {
	User, Pass, GAIAID, Contact string // login credentials
	NormalizedUser              string // user with domain added, periods removed, etc.
	ParentUser, ParentPass      string // unicorn parent login credentials
	KeepState                   bool
	DeferLogin                  bool
	LoginMode                   LoginMode
	ReuseSession                bool // reuse exiting login session if configuration matches
	EnableLoginVerboseLogs      bool // enable verbose logging in some login related files
	VKEnabled                   bool
	SkipOOBEAfterLogin          bool // skip OOBE post user login
	InstallWebApp               bool // auto install essential apps after user login
	Region                      string
	PolicyEnabled               bool   // flag to enable policy fetch
	DMSAddr                     string // Device Management URL, or empty if using default
	Enroll                      bool   // whether device should be enrolled
	ARCMode                     ARCMode
	RestrictARCCPU              bool // a flag to control cpu restrictions on ARC

	// If BreakpadTestMode is true, tell Chrome's breakpad to always write
	// dumps directly to a hardcoded directory.
	BreakpadTestMode bool
	ExtraArgs        []string
	EnableFeatures   []string
	DisableFeatures  []string

	ExtraExtDirs []string // directories containing all extra unpacked extensions to load
	SigninExtKey string   // private key for signin profile test extension manifest
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
