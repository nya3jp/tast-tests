// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policy"
	"chromiumos/tast/local/policy/fakedms"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Enroll,
		Desc:         "Client side of the enroll test example",
		Contacts:     []string{},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"informational"},
	})
}

func Enroll(ctx context.Context, s *testing.State) {
	// Start FakeDMS.
	fdms, err := fakedms.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}
	defer fdms.Stop(ctx)

	p := &policy.DeviceAllowBluetooth{Val: true}
	pb := fakedms.NewPolicyBlob()
	pb.AddPolicy(p)

	if err = fdms.WritePolicyBlob(pb); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	authOpt := chrome.Auth("tast-user@managedchrome.com", "test0000", "gaia-id")
	cr, err := chrome.New(ctx, authOpt, chrome.DMSPolicy(fdms.URL), chrome.EnterpriseEnroll())
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	// Fetch the policies from the DUT and verify the device policy is set.
	pols, err := policy.PoliciesFromDUT(ctx, cr)
	testing.ContextLog(ctx, "See policy set here: ", pols)
	actual, ok := pols.Chrome[p.Name()]
	if !ok {
		testing.ContextLog(ctx, "Policy missing")
	}

	if string(actual.Value) != "true" {
		s.Fatal("Policy did not set")
	}
}
