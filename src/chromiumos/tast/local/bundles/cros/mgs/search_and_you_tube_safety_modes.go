// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mgs

import (
	"context"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/mgs"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SearchAndYouTubeSafetyModes,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Verify behavior of ForceGoogleSafeSearch and ForceYouTubeRestrict policies on Managed Guest Session",
		Contacts: []string{
			"cmfcmf@google.com", // Test author
			"chromeos-kiosk-eng+TAST@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.FakeDMSEnrolled,
	})
}

func SearchAndYouTubeSafetyModes(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Launch a new MGS with default account.
	mgs, cr, err := mgs.New(
		ctx,
		fdms,
		mgs.DefaultAccount(),
		mgs.AutoLaunch(mgs.MgsAccountID),
		mgs.AddPublicAccountPolicies(mgs.MgsAccountID, []policy.Policy{
			&policy.ForceGoogleSafeSearch{Val: true},
			&policy.ForceYouTubeRestrict{Val: 2}, // TODO: magic number
		}),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome on Signin screen with default MGS account: ", err)
	}
	defer func() {
		if err := mgs.Close(ctx); err != nil {
			s.Fatal("Failed close MGS: ", err)
		}
	}()

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	// TODO
	isGoogleSafe(ctx, s, cr)

	// isGoogleSafe(ctx, s, br)
	// if isSafe != param.wantSafe {
	// 	s.Errorf("Unexpected safe search behavior; got %t, want %t", isSafe, param.wantSafe)
	// }
}

func isGoogleSafe(ctx context.Context, s *testing.State, cr *chrome.Chrome) bool {
	conn, err := cr.NewConn(ctx, "https://www.google.com/search?q=kittens")
	if err != nil {
		s.Fatal("Failed to connect to chrome: ", err)
	}
	defer conn.Close()

	var isSafe bool
	if err := conn.Eval(ctx, `new URL(document.URL).searchParams.get("safe") == "active"`, &isSafe); err != nil {
		s.Fatal("Could not read safe search param from URL: ", err)
	}
	return isSafe
}
