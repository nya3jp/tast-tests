// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"context"
	"strconv"

	nearbycommon "chromiumos/tast/common/cros/nearbyshare"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OnboardingInitialPageUI,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that Nearby Share can be enabled from the initial page of onboarding workflow",
		Contacts: []string{
			"pushi@google.com",
			"chromeos-sw-engprod@google.com",
		},
		// Use this variable to preserve user accounts on the DUT when running locally,
		// i.e. tast run -var=keepState=true <dut> nearbyshare.OnboardingSinglePageUI
		Vars:         []string{nearbycommon.KeepStateVar},
		Attr:         []string{"group:nearby-share"},
		SoftwareDeps: []string{"chrome"},
	})
}

// OnboardingInitialPageUI tests that we can enable Nearby Share in the initial onboarding page.
func OnboardingInitialPageUI(ctx context.Context, s *testing.State) {
	opts := []chrome.Option{
		// Enable the feature flag for nearby one-page onboarding workflow
		// TODO(crbug.com/1265562): Remove after we fully launch the feature
		chrome.EnableFeatures("NearbySharingOnePageOnboarding"),
		chrome.ExtraArgs("--nearby-share-verbose-logging"),
	}
	if val, ok := s.Var(nearbycommon.KeepStateVar); ok {
		b, err := strconv.ParseBool(val)
		if err != nil {
			s.Fatalf("Unable to convert %v var to bool: %v", nearbycommon.KeepStateVar, err)
		}
		if b {
			opts = append(opts, chrome.KeepState())
		}
	}
	cr, err := chrome.New(
		ctx,
		opts...,
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	if err := nearbyshare.EnableNearbyShareInInitialOnboardingPage(ctx, tconn, cr); err != nil {
		s.Fatal("Failed to enable Nearby Share in the initial page of onboarding workflow: ", err)
	}
}
