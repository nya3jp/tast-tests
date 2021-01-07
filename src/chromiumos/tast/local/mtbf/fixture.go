// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mtbf implements a library used for MTBF testing.
package mtbf

import (
	"context"

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

func chromeLoginReuseOptions(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
	var userID, userPasswd string
	var ok bool
	userID, ok = s.Var(userVar)
	if !ok {
		s.Fatalf("create new Chrome - %s not provided. Please specify it in your test vars configuration", userVar)
	}
	userPasswd, ok = s.Var(passwdVar)
	if !ok {
		s.Fatalf("create new Chrome - %s not provided. Please specify it in your test vars configuration", passwdVar)
	}

	var chromeOpts = []chrome.Option{
		chrome.DisableFeatures("CameraSystemWebApp"),
		chrome.KeepState(),
		chrome.ARCSupported(),
		chrome.GAIALogin(),
		chrome.ReuseSession(),               // Indicate to reuse login if possible.
		chrome.Auth(userID, userPasswd, ""), // Use runtime provided credentials
	}
	return chromeOpts, nil
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            ChromeLoginReuseFixture,
		Desc:            "Reuse the existing user login session",
		Contacts:        []string{"xliu@cienet.com"},
		Impl:            chrome.NewLoggedInFixture(chromeLoginReuseOptions),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{userVar, passwdVar},
	})
}
