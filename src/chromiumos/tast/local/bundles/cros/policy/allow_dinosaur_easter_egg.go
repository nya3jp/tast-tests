// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
			{Name: "unset", Val: &policy.AllowDinosaurEasterEgg{Stat: policy.StatusUnset}},
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
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

	// Create a policy blob and have the FakeDMS serve it.
	pb := fakedms.NewPolicyBlob()
	pb.AddPolicies([]policy.Policy{p})
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
	if isBlocked && !(p.Stat == policy.StatusUnset) && p.Val {
		s.Fatal("Incorrect behavior: Dinosaur game was blocked")
	}
	// False or Unset: game is blocked.
	if !isBlocked && ((p.Stat == policy.StatusUnset) || !p.Val) {
		s.Fatal("Incorrect behavior: Dinosaur game was not blocked")
	}
}
