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
		Func:         FakeDMS,
		Desc:         "Example of a policy test using FakeDMS",
		Contacts:     []string{"kathrelkeld@chromium.org"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"informational"},
	})
}

func FakeDMS(ctx context.Context, s *testing.State) {
	// Start FakeDMS.
	fdms, err := fakedms.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}
	defer fdms.Stop(ctx)

	// Create a policy blob and have the FakeDMS serve it.
	pb := policy.NewPolicyBlob()
	pb.AddUserPolicy("HighContrastEnabled", true)
	pb.AddUserPolicy("RemoteAccessHostClientDomainList",
		[]string{"domaina.com", "domainb.com"})
	if err = fdms.SetPolicyBlob(pb); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	// Note that only some domains will work - use managedchrome.com even
	// if you are not using Gaia login.
	authOpt := chrome.Auth("tast-user@managedchrome.com", "1234", "gaia-id")
	cr, err := chrome.New(ctx, authOpt, chrome.DMSUrl(fdms.URL))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	// Verify that policies were correctly set.
	if err := policy.VerifyPolicyBlob(ctx, cr, pb); err != nil {
		s.Fatal("Policies were not set properly on the DUT: ", err)
	}
}
