// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"go.chromium.org/chromiumos/tast-tests/common/fixture"
	"go.chromium.org/chromiumos/tast-tests/common/policy"
	"go.chromium.org/chromiumos/tast-tests/common/policy/fakedms"
	"go.chromium.org/chromiumos/tast/ctxutil"
	"go.chromium.org/chromiumos/tast-tests/local/chrome"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/browser"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/browser/browserfixt"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/checked"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/faillog"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/nodewith"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/uiauto/role"
	"go.chromium.org/chromiumos/tast-tests/local/input"
	"go.chromium.org/chromiumos/tast-tests/local/policyutil"
	"go.chromium.org/chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PrintingBackgroundGraphicsDefault,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checking if the 'Background graphics' option is set by default depending on the value of this policy",
		Contacts: []string{
			"cmfcmf@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
	})
}

// PrintingBackgroundGraphicsDefault tests the PrintingBackgroundGraphicsDefault policy.
func PrintingBackgroundGraphicsDefault(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get the keyboard: ", err)
	}
	defer kb.Close()

	for _, param := range []struct {
		name        string
		wantChecked checked.Checked
		policy      *policy.PrintingBackgroundGraphicsDefault
	}{
		{
			name:        "enabled",
			wantChecked: checked.True,
			policy:      &policy.PrintingBackgroundGraphicsDefault{Val: "enabled"},
		},
		{
			name:        "disabled",
			wantChecked: checked.False,
			policy:      &policy.PrintingBackgroundGraphicsDefault{Val: "disabled"},
		},
		{
			name:        "unset",
			wantChecked: checked.False,
			policy:      &policy.PrintingBackgroundGraphicsDefault{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Reserve 10 seconds for cleanup.
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer cancel()

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Setup browser based on the chrome type.
			conn, _, closeBrowser, err := browserfixt.SetUpWithURL(ctx, cr, s.Param().(browser.Type), chrome.BlankURL)
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)
			defer conn.Close()
			// The UI tree must be dumped before closing the browser.
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Connect to Test API to use it with the UI library.
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create Test API connection: ", err)
			}

			checkbox := nodewith.Role(role.CheckBox).Name("Background graphics")

			ui := uiauto.New(tconn)
			if err := uiauto.Combine("open print preview",
				kb.AccelAction("Ctrl+P"),
				ui.LeftClick(nodewith.Role(role.Button).Name("More settings")),
				ui.WaitUntilExists(checkbox),
			)(ctx); err != nil {
				s.Fatal("Failed to open print preview: ", err)
			}
			nodeInfo, err := ui.Info(ctx, checkbox)
			if err != nil {
				s.Fatal("Failed to check state of 'Background graphics' checkbox: ", err)
			}

			if nodeInfo.Checked != param.wantChecked {
				s.Errorf("Unexpected state of the 'Background graphics' checkbox: got %s; want %s", nodeInfo.Checked, param.wantChecked)
			}
		})
	}
}
