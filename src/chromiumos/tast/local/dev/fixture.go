// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dev provides fixtures needed for dev.* tests.
package dev

import (
	"context"
	"strconv"
	"strings"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

// Fixtures used by the dev package
const (
	// ChromeLoggedIn is a fixture name.
	ChromeLoggedIn = "devChromeLoggedIn"
	// LacrosLoggedIn is a fixture name.
	LacrosLoggedIn = "devLacrosLoggedIn"
)

// rdpVars represents the configurable parameters for the remote desktop session.
type rdpVars struct {
	user    string
	pass    string
	contact string
	// wait      bool
	reset     bool
	extraArgs []string
}

// getVars extracts the testing parameters from testing.FixtState. The user
// provided credentials would override the credentials from config file.
func getVars(s *testing.FixtState) rdpVars {
	user, hasUser := s.Var("user")
	if !hasUser {
		user = s.RequiredVar("dev.username")
	}

	pass, hasPass := s.Var("pass")
	if !hasPass {
		pass = s.RequiredVar("dev.password")
	}

	contact, ok := s.Var("contact")
	if !ok {
		contact = ""
	}

	resetStr, ok := s.Var("reset")
	if !ok {
		resetStr = "false"
	}
	reset, err := strconv.ParseBool(resetStr)
	if err != nil {
		s.Fatal("Failed to parse the variable `reset`: ", err)
	}

	extraArgsStr, ok := s.Var("extra_args")
	if !ok {
		extraArgsStr = ""
	}
	extraArgs := strings.Fields(extraArgsStr)

	return rdpVars{
		user:    user,
		pass:    pass,
		contact: contact,
		// wait:      wait,
		reset:     reset,
		extraArgs: extraArgs,
	}
}

func defaultOpts(vars rdpVars) []chrome.Option {
	var opts []chrome.Option

	chromeARCOpt := chrome.ARCDisabled()
	if arc.Supported() {
		chromeARCOpt = chrome.ARCSupported()
	}
	opts = append(opts, chromeARCOpt)
	opts = append(opts, chrome.GAIALogin(chrome.Creds{
		User:    vars.user,
		Pass:    vars.pass,
		Contact: vars.contact,
	}))

	if !vars.reset {
		opts = append(opts, chrome.KeepState())
	}

	if len(vars.extraArgs) > 0 {
		opts = append(opts, chrome.ExtraArgs(vars.extraArgs...))
	}
	return opts
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     "devChromeLoggedIn",
		Desc:     "Logged into a user session for dev package",
		Contacts: []string{"hyungtaekim@chromium.org", "shik@chromium.org", "tast-users@chromium.org"},

		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return defaultOpts(getVars(s)), nil
		}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars: []string{
			// For running manually.
			"user", "pass", "contact", "extra_args", "wait", "reset",
			// For automated testing.
			// Note that VarDeps is not supported in fixture
			"dev.username", "dev.password",
		},
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "devLacrosLoggedIn",
		Desc:     "Logged into a user session for dev package with Lacros Primary",
		Contacts: []string{"hyungtaekim@chromium.org", "shik@chromium.org", "tast-users@chromium.org"},
		Impl: lacrosfixt.NewFixture(lacrosfixt.Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return append(defaultOpts(getVars(s)),
				chrome.EnableFeatures("LacrosPrimary"),
				chrome.ExtraArgs("--disable-lacros-keep-alive", "--disable-login-lacros-opening")), nil
		}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars: []string{
			// For running manually.
			"user", "pass", "contact", "extra_args", "wait", "reset",
			// For automated testing.
			// Note that VarDeps is not supported in fixture
			"dev.username", "dev.password",
			// For various lacros binary source
			lacrosfixt.LacrosDeployedBinary,
		},
	})
}
