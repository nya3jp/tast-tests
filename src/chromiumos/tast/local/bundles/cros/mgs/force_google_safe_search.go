// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mgs

import (
	"context"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/mgs"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ForceGoogleSafeSearch,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Verify behavior of ForceGoogleSafeSearch policy on Managed Guest Session",
		Contacts: []string{
			"cmfcmf@google.com", // Test author
			"chromeos-kiosk-eng+TAST@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.FakeDMSEnrolled,
	})
}

func ForceGoogleSafeSearch(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	for _, param := range []struct {
		name     string
		wantSafe bool
		value    *policy.ForceGoogleSafeSearch
	}{
		{
			name:     "enabled",
			wantSafe: true,
			value:    &policy.ForceGoogleSafeSearch{Val: true},
		},
		{
			name:     "disabled",
			wantSafe: false,
			value:    &policy.ForceGoogleSafeSearch{Val: false},
		},
		{
			name:     "unset",
			wantSafe: false,
			value:    &policy.ForceGoogleSafeSearch{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Launch a new MGS with default account.
			mgs, cr, err := mgs.New(
				ctx,
				fdms,
				mgs.DefaultAccount(),
				mgs.AutoLaunch(mgs.MgsAccountID),
				mgs.AddPublicAccountPolicies(mgs.MgsAccountID, []policy.Policy{param.value}),
			)
			if err != nil {
				s.Fatal("Failed to start Chrome on Signin screen with default MGS account: ", err)
			}
			defer func() {
				if err := mgs.Close(ctx); err != nil {
					s.Fatal("Failed close MGS: ", err)
				}
			}()
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_")
			br := cr.Browser()

			// Run actual test.
			if err := testGoogleSafeSearch(ctx, br, param.wantSafe); err != nil {
				s.Error("Failed to verify state of Google safe search: ", err)
			}
		})
	}
}

func testGoogleSafeSearch(ctx context.Context, br *browser.Browser, safeSearchExpected bool) error {
	conn, err := br.NewConn(ctx, "https://www.google.com/search?q=kittens")
	if err != nil {
		return errors.Wrap(err, "failed to connect to Chrome")
	}
	defer conn.Close()

	var isSafe bool
	if err := conn.Eval(ctx, `new URL(document.URL).searchParams.get("safe") == "active"`, &isSafe); err != nil {
		return errors.Wrap(err, "could not read safe search param from URL")
	}

	if isSafe != safeSearchExpected {
		return errors.Errorf("unexpected safe search behavior; got %t, want %t", isSafe, safeSearchExpected)
	}

	return nil
}
