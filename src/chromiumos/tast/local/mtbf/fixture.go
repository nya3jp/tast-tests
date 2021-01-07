// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mtbf implements a library used for MTBF testing.
package mtbf

import (
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	// username and password runtime variable name expected when running mtbf tests.
	userVar   = "userID"
	passwdVar = "userPasswd"

	// ChromeLoginReuseFixture is a fixture name that will be registered to tast.
	ChromeLoginReuseFixture = "chromeLoginReuse"
)

// Chrome options for loginReuse fixture.
var chromeOpts = []chrome.Option{
	chrome.DisableFeatures("CameraSystemWebApp"),
	chrome.KeepState(),
	chrome.ARCSupported(),
	chrome.GAIALogin(),
	chrome.ReuseLogin(), // Indicate to reuse login if possible.
}

var fixtureOpts = []chrome.FixtureOption{
	chrome.LoggedInUser(userVar, passwdVar),
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            ChromeLoginReuseFixture,
		Desc:            "Reuse the existing user login session",
		Contacts:        []string{"xliu@cienet.com"},
		Impl:            chrome.NewLoggedInFixtureWithOptions(chromeOpts, fixtureOpts),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{userVar, passwdVar},
	})
}
