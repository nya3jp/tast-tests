// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fixtures

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	// usernameVar stores the required username variable.
	usernameVar = "arcappgameperf.username"

	// passwordVar stores the required username variable.
	passwordVar = "arcappgameperf.password"

	// ARCAppGamePerfFixture is a fixture name that will be registered to tast.
	// The fixture brings up Chrome and ARC with the Play Store opted in to.
	ARCAppGamePerfFixture = "arcAppGamePerfFixture"

	// resetTimeout indicates how long the fixture has to reset, and tear down.
	resetTimeout = 30 * time.Second
)

// arcAppGamePerfFixtureOptions sets up a fixture which sets up Chrome, ARC,
// opts in to the Play Store, and logs in with the provided credentials.
func arcAppGamePerfFixtureOptions(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
	return []chrome.Option{
		chrome.ExtraArgs(arc.DisableSyncFlags()...),
		chrome.GAIALogin(chrome.Creds{User: s.RequiredVar(usernameVar), Pass: s.RequiredVar(passwordVar)})}, nil
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            ARCAppGamePerfFixture,
		Desc:            "The fixture starts chrome with ARC supported",
		Contacts:        []string{"davidwelling@google.com", "arc-engprod@google.com"},
		Impl:            arc.NewArcBootedWithPlayStoreFixture(arcAppGamePerfFixtureOptions),
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute,
		ResetTimeout:    resetTimeout,
		PostTestTimeout: arc.PostTestTimeout,
		TearDownTimeout: resetTimeout,
		Vars:            []string{usernameVar, passwordVar},
	})
}
