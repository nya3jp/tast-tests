// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WebAppWithPolicies,
		Desc: "Start Kiosk application with other public account policies to be applied",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
			"alt-modalities-stability@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "fakeDMSEnrolled",
	})
}

func WebAppWithPolicies(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)
	cr, err := chrome.New(
		ctx,
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment(),
	)
	if err != nil {
		s.Error("Failed to start Chrome: ", err)
	}

	defer func(ctx context.Context) {
		// Use cr as a reference to close the last started Chrome instance.
		if err := cr.Close(ctx); err != nil {
			s.Error("Failed to close Chrome connection: ", err)
		}
	}(ctx)

	// Update policies.
	webKioskAccountID := "1"
	webKioskAccountType := policy.AccountTypeKioskWebApp
	webKioskIconURL := "https://www.google.com"
	webKioskTitle := "TastKioskModeSetByPolicyGooglePage"
	webKioskURL := "https://www.google.com"

	// Need to mix policies that way, otherwise enterprise policies don't
	// get apply to local public account.
	policies := []policy.Policy{
		&policy.DeviceLocalAccounts{
			Val: []policy.DeviceLocalAccountInfo{
				{
					AccountID:   &webKioskAccountID,
					AccountType: &webKioskAccountType,
					WebKioskAppInfo: &policy.WebKioskAppInfo{
						Url:     &webKioskURL,
						Title:   &webKioskTitle,
						IconUrl: &webKioskIconURL,
					},
				},
			},
		},
		&policy.DeviceLocalAccountAutoLoginId{
			Val: webKioskAccountID,
		},
	}

	pb := fakedms.NewPolicyBlob()
	pb.AddPolicies(policies)
	pb.AddPublicAccountPolicy(webKioskAccountID,
		&policy.FloatingAccessibilityMenuEnabled{
			Val: true,
		},
		// &policy.VirtualKeyboardFeatures{
		// 	Val: &policy.VirtualKeyboardFeaturesValue{
		// 		AutoCompleteEnabled: true,
		// 		AutoCorrectEnabled:  true,
		// 		HandwritingEnabled:  true,
		// 		SpellCheckEnabled:   true,
		// 		VoiceInputEnabled:   true,
		// 	},
		// },
	)

	if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		s.Fatal("Failed to serve policies: ", err)
	}

	// Close the previous Chrome instance.
	if err := cr.Close(ctx); err != nil {
		s.Error("Failed to close Chrome connection: ", err)
	}

	// Restart Chrome.
	cr, err = chrome.New(ctx,
		chrome.NoLogin(),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment(),
	)
	if err != nil {
		s.Fatal("Chrome restart failed: ", err)
	}

	testing.Sleep(ctx, time.Minute)
	// TODO => How to check that accessibility float exist? Image machting (ACUITI) for now?
}
