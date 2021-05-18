// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package familylink

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/familylink"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PolicyLogin,
		Desc:         "Checks if Unicorn login with policy setup is working",
		Contacts:     []string{"xiqiruan@chromium.org", "cros-families-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      chrome.GAIALoginTimeout + 5*time.Minute,
		Vars: []string{
			"unicorn.childUser",
		},
		Fixture: "familyLinkUnicornPolicyLogin",
	})
}

func PolicyLogin(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*familylink.FixtData).FakeDMS
	cr := s.FixtValue().(*familylink.FixtData).Chrome
	tconn := s.FixtValue().(*familylink.FixtData).TestConn
	if tconn == nil {
		s.Fatal("Failed to create test API connection")
	}
	if fdms == nil {
		s.Fatal("Failed to start fake DM server")
	}
	if cr == nil {
		s.Fatal("Failed to start Chrome")
	}

	policies := []policy.Policy{
		&policy.ForceGoogleSafeSearch{Val: true},
	}

	pb := &fakedms.PolicyBlob{
		PolicyUser:       s.RequiredVar("unicorn.childUser"),
		ManagedUsers:     []string{"*"},
		InvalidationSrc:  16,
		InvalidationName: "test_policy",
	}
	pb.AddPolicies(policies)

	if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		s.Fatal("Failed to serve policies: ", err)
	}

	s.Log("Verifying policies was delivered to device")
	if err := policyutil.Verify(ctx, tconn, policies); err != nil {
		s.Fatal("Failed to verify policies: ", err)
	}
}
