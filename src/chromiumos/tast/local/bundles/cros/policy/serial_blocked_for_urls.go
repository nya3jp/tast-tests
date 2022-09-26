// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/policy/serial"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SerialBlockedForUrls,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests the behavior of the SerialBlockedForUrls policy by checking that it correctly configures access to the serial port selection prompt",
		Contacts: []string{
			"cmfcmf@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{
			{
				Fixture: fixture.ChromePolicyLoggedIn,
				Val:     browser.TypeAsh,
			}, {
				Name:              "lacros",
				Fixture:           fixture.LacrosPolicyLoggedIn,
				Val:               browser.TypeLacros,
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"lacros"},
			},
		},
		Data: []string{serial.SerialTestPage},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.DefaultSerialGuardSetting{}, pci.VerifiedFunctionalityUI),
			pci.SearchFlag(&policy.SerialBlockedForUrls{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// SerialBlockedForUrls tests the SerialBlockedForUrls policy.
func SerialBlockedForUrls(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	httpServer := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer httpServer.Close()

	for _, param := range []struct {
		name             string
		wantSerialDialog bool
		policies         []policy.Policy
	}{
		{
			name:             "set",
			wantSerialDialog: false,
			policies: []policy.Policy{
				&policy.SerialBlockedForUrls{Val: []string{httpServer.URL}}},
		},
		{
			name:             "set_and_ask_by_default",
			wantSerialDialog: false,
			policies: []policy.Policy{
				&policy.DefaultSerialGuardSetting{Val: serial.DefaultSerialGuardSettingAsk},
				&policy.SerialBlockedForUrls{Val: []string{httpServer.URL}}},
		},
		{
			name:             "set_and_block_by_default",
			wantSerialDialog: false,
			policies: []policy.Policy{
				&policy.DefaultSerialGuardSetting{Val: serial.DefaultSerialGuardSettingBlock},
				&policy.SerialBlockedForUrls{Val: []string{httpServer.URL}}},
		},
		{
			name:             "set_non_matching_and_ask_by_default",
			wantSerialDialog: true,
			policies: []policy.Policy{
				&policy.DefaultSerialGuardSetting{Val: serial.DefaultSerialGuardSettingAsk},
				&policy.SerialBlockedForUrls{Val: []string{"https://example.com"}}},
		},
		// Conflicts with the SerialAskForUrls policy are tested as part of that policy test.
		{
			name:             "unset",
			wantSerialDialog: true,
			policies: []policy.Policy{
				&policy.SerialBlockedForUrls{Stat: policy.StatusUnset}},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Reserve ten seconds for cleanup.
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer cancel()

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, param.policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Setup browser based on the chrome type.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			if err := serial.TestSerialPortRequest(ctx, cr, br, httpServer.URL, param.wantSerialDialog); err != nil {
				s.Fatal("Failed while testing serial port request: ", err)
			}
		})
	}
}
