// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kiosk

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/kioskmode"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FloatingAccessibilityMenuEnabled,
		Desc: "Start Kiosk application with FloatingAccessibilityMenuEnabled applied to the account",
		Contacts: []string{
			"kamilszarek@google.com", // Test author
			"alt-modalities-stability@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "fakeDMSEnrolled",
	})
}

func FloatingAccessibilityMenuEnabled(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)
	cr, err := kioskmode.New(
		ctx,
		fdms,
		kioskmode.DefaultLocalAccounts(),
		kioskmode.PublicAccountPolicies(kioskmode.WebKioskAccountID, []policy.Policy{&policy.FloatingAccessibilityMenuEnabled{Val: true}}),
		kioskmode.AutoLaunch(kioskmode.WebKioskAccountID),
	)
	if err != nil {
		s.Error("Failed to start Chrome in Kiosk mode: ", err)
	}

	defer cr.Close(ctx)
	defer policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{})

	testConn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "kiosk_with_FloatingAccessibilityMenuEnabled")

	ui := uiauto.New(testConn)
	if err := ui.WaitUntilExists(nodewith.Name("Floating accessibility menu").HasClass("Widget"))(ctx); err != nil {
		s.Error("Failed to find floating accessibility menu: ", err)
	}
}
