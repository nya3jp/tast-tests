// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policy"
	"chromiumos/tast/local/policy/fakedms"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     AllowDinosaurEasterEgg,
		Desc:     "Behavior of AllowDinosaurEasterEgg policy",
		Contacts: []string{},
		Params: []testing.Param{
			{Name: "true", Val: &policy.AllowDinosaurEasterEgg{Val: true}},
			{Name: "false", Val: &policy.AllowDinosaurEasterEgg{Val: false}},
			{Name: "unset", Val: &policy.AllowDinosaurEasterEgg{Stat: policy.UnsetStatus}},
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"informational"},
	})
}

func AllowDinosaurEasterEgg(ctx context.Context, s *testing.State) {
	// Start FakeDMS.
	fdms, err := fakedms.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}
	defer fdms.Stop(ctx)

	p := s.Param().(*policy.AllowDinosaurEasterEgg)

	// TODO: the below is for testing purposes only and will not be checked in.
	ps := []policy.Policy{p}
	ps = append(ps, &policy.WallpaperImage{Val: &policy.WallpaperImageValue{
		Url:  "https://example.com/wallpaper.jpg",
		Hash: "baddecafbaddecafbaddecafbaddecafbaddecafbaddecafbaddecafbaddecaf"}})
	ps = append(ps, &policy.RemoteAccessHostUdpPortRange{Val: "0-1000000"})
	ps = append(ps, &policy.UsageTimeLimit{Val: &policy.UsageTimeLimitValue{
		Overrides: []*policy.UsageTimeLimitValueOverrides{{
			Action:          "UNLOCK",
			CreatedAtMillis: "1250000",
			ActionSpecificData: &policy.UsageTimeLimitValueOverridesActionSpecificData{
				DurationMins: 30,
			},
		}},
		TimeWindowLimit: &policy.UsageTimeLimitValueTimeWindowLimit{
			Entries: []*policy.UsageTimeLimitValueTimeWindowLimitEntries{{
				EffectiveDay: "WEDNESDAY",
				StartsAt:     &policy.RefTime{Hour: 21, Minute: 4},
				EndsAt:       &policy.RefTime{Hour: 7, Minute: 30},
			}},
		},
		TimeUsageLimit: &policy.UsageTimeLimitValueTimeUsageLimit{
			ResetAt: &policy.RefTime{Hour: 6, Minute: 30},
			Sunday: &policy.RefTimeUsageLimitEntry{
				UsageQuotaMins:    120,
				LastUpdatedMillis: "1200000",
			},
			Monday: &policy.RefTimeUsageLimitEntry{
				UsageQuotaMins:    120,
				LastUpdatedMillis: "1200000",
			},
			Tuesday: &policy.RefTimeUsageLimitEntry{
				UsageQuotaMins:    120,
				LastUpdatedMillis: "1200000",
			},
			Wednesday: &policy.RefTimeUsageLimitEntry{
				UsageQuotaMins:    120,
				LastUpdatedMillis: "1200000",
			},
			Thursday: &policy.RefTimeUsageLimitEntry{
				UsageQuotaMins:    120,
				LastUpdatedMillis: "1200000",
			},
			Friday: &policy.RefTimeUsageLimitEntry{
				UsageQuotaMins:    120,
				LastUpdatedMillis: "1200000",
			},
			Saturday: &policy.RefTimeUsageLimitEntry{
				UsageQuotaMins:    120,
				LastUpdatedMillis: "1200000",
			},
		},
	}})

	// Create a policy blob and have the FakeDMS serve it.
	pb := fakedms.NewPolicyBlob()
	pb.AddPolicies(ps)
	if err = fdms.WritePolicyBlob(pb); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	authOpt := chrome.Auth("tast-user@managedchrome.com", "test0000", "gaia-id")
	cr, err := chrome.New(ctx, authOpt, chrome.DMSPolicy(fdms.URL))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	// Set up Chrome Test API connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	if err := policy.VerifyPoliciesSet(ctx, tconn, ps); err != nil {
		s.Fatal("Policies were not set properly on the DUT: ", err)
	}

	// Run actual test.
	const url = "chrome://dino"
	if err := tconn.Navigate(ctx, url); err != nil {
		s.Fatal("Could not open ", url, err)
	}

	var content json.RawMessage
	query := `document.querySelector('* /deep/ #main-frame-error div.snackbar')`
	if err = tconn.Eval(ctx, query, &content); err != nil {
		s.Fatal("Could not read from dino page: ", err)
	}
	isBlocked := (string(content) == "{}")

	// Set to True: game is allowed.
	if isBlocked && !(p.Stat == policy.UnsetStatus) && p.Val {
		s.Fatal("Incorrect behavior: Dinosaur game was blocked")
	}
	// False or Unset: game is blocked.
	if !isBlocked && ((p.Stat == policy.UnsetStatus) || !p.Val) {
		s.Fatal("Incorrect behavior: Dinosaur game was not blocked")
	}
}
