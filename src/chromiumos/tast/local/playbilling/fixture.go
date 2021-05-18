// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package playbilling

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	userVar = "arc.PlayBillingUsername"
	passVar = "arc.PlayBillingPassword"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     "arcBootedForPlayBilling",
		Desc:     "The fixture starts chrome with ARC supported used for Play Billing tests",
		Contacts: []string{"benreich@chromium.org", "jshikaram@chromium.org"},
		Impl: arc.NewArcBootedWithPlayStoreFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			userID, ok := s.Var(userVar)
			if !ok {
				s.Fatalf("Runtime variable %s is not provided", userVar)
			}
			userPasswd, ok := s.Var(passVar)
			if !ok {
				s.Fatalf("Runtime variable %s is not provided", passVar)
			}

			return []chrome.Option{
				chrome.ExtraArgs(arc.DisableSyncFlags()...),
				chrome.ARCSupported(),
				chrome.GAIALogin(chrome.Creds{User: userID, Pass: userPasswd})}, nil
		}),
		// Add two minutes to setup time to allow extra Play Store UI operations.
		SetUpTimeout: chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute,
		ResetTimeout: chrome.ResetTimeout,
		// Provide a longer enough PostTestTimeout value to fixture when ARC will try to dump ARCVM message.
		// Or there might be error of "context deadline exceeded".
		PostTestTimeout: 5 * time.Second,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{userVar, passVar},
	})
}
