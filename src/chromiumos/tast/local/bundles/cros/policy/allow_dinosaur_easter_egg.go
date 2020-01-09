// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policy"
	"chromiumos/tast/local/policy/fakedms"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AllowDinosaurEasterEgg,
		Desc: "Behavior of AllowDinosaurEasterEgg policy",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"kathrelkeld@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func AllowDinosaurEasterEgg(ctx context.Context, s *testing.State) {
	checkAllowDinosaurEasterEgg := func(ctx context.Context, s *testing.State, value *policy.AllowDinosaurEasterEgg) {
		// Start FakeDMS.
		fdms, err := fakedms.New(ctx, s.OutDir())
		if err != nil {
			s.Fatal("Failed to start FakeDMS: ", err)
		}
		defer fdms.Stop(ctx)

		// Create a policy blob and have the FakeDMS serve it.
		pb := fakedms.NewPolicyBlob()
		pb.AddPolicies([]policy.Policy{value})
		if err = fdms.WritePolicyBlob(pb); err != nil {
			s.Fatal("Failed to write policies to FakeDMS: ", err)
		}

		// Start a Chrome instance that will fetch policies from the FakeDMS.
		cr, err := chrome.New(ctx,
			chrome.Auth("tast-user@managedchrome.com", "test0000", "gaia-id"),
			chrome.DMSPolicy(fdms.URL))
		if err != nil {
			s.Fatal("Chrome login failed: ", err)
		}
		defer cr.Close(ctx)

		// Set up Chrome Test API connection.
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to create Test API connection: ", err)
		}

		// Run actual test.
		const url = "chrome://dino"
		if err := tconn.Navigate(ctx, url); err != nil {
			s.Fatalf("Could not open %q: %v", url, err)
		}

		var isBlocked bool
		if err = tconn.Eval(ctx, `document.querySelector('* /deep/ #main-frame-error div.snackbar') === null`, &isBlocked); err != nil {
			s.Fatal("Could not read from dino page: ", err)
		}

		expectedBlocked := !(value.Stat == policy.StatusUnset) && value.Val

		if isBlocked != expectedBlocked {
			s.Errorf("Unexpected blocked behavior: got %t; want %t", isBlocked, expectedBlocked)
		}
	}

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// value is the policy value.
		value *policy.AllowDinosaurEasterEgg
	}{
		{
			name:  "true",
			value: &policy.AllowDinosaurEasterEgg{Val: true},
		},
		{
			name:  "false",
			value: &policy.AllowDinosaurEasterEgg{Val: false},
		},
		{
			name:  "unset",
			value: &policy.AllowDinosaurEasterEgg{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			checkAllowDinosaurEasterEgg(ctx, s, param.value)
		})
	}
}
