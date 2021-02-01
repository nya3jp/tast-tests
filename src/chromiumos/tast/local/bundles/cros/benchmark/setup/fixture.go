// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	// userVar and passwdVar are runtime variable names for user login credentials.
	userVar   = "benchmark.username"
	passwdVar = "benchmark.password"

	// BenchmarkARCFixture is a fixture name that will be registered to tast.
	// The fxture brings up Chrome and ARC with PlayStore.
	BenchmarkARCFixture = "benchmarkARCFixture"
)

func benchmarkARCFixtureOptions(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
	userID, ok := s.Var(userVar)
	if !ok {
		s.Fatalf("Runtime variable %s is not provided", userVar)
	}
	userPasswd, ok := s.Var(passwdVar)
	if !ok {
		s.Fatalf("Runtime variable %s is not provided", passwdVar)
	}

	return []chrome.Option{
		chrome.KeepState(),
		chrome.ARCSupported(),
		chrome.GAIALogin(chrome.Creds{User: userID, Pass: userPasswd})}, nil
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     BenchmarkARCFixture,
		Desc:     "The fixture starts chrome with ARC supported",
		Contacts: []string{"xliu@cienet.com"},
		Impl:     arc.NewArcBootedWithPlayStoreFixture(benchmarkARCFixtureOptions),
		// Add two minutes to setup time to allow extra Play Store UI operations.
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{userVar, passwdVar},
	})
}
