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
	mtbfUserVar   = "userID"
	mtbfPasswdVar = "userPasswd"

	// ChromeLoginReuseFixture is a fixture name that will be registered to tast.
	ChromeLoginReuseFixture = "chromeLoginReuse"
)

// Chrome options for loginReuse fixture.
var loginReuseOpts = []chrome.Option{
	chrome.DisableFeatures("CameraSystemWebApp"),
	chrome.KeepState(),
	chrome.ARCSupported(),
	chrome.GAIALogin(),
	chrome.ReuseLogin(), // Indicate to reuse login if possible.
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            ChromeLoginReuseFixture,
		Desc:            "Reuse the existing user login session",
		Contacts:        []string{"xliu@cienet.com"},
		Impl:            chrome.NewLoggedInUserFixture(mtbfUserVar, mtbfPasswdVar, loginReuseOpts...),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{mtbfUserVar, mtbfPasswdVar},
	})
}
