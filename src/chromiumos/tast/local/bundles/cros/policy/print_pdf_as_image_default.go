// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PrintPdfAsImageDefault,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checking if the 'Print as image' option is set by default depending on the value of this policy",
		Contacts: []string{
			"cmfcmf@google.com", // Test author
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
		Data: []string{"print_pdf_as_image_default.pdf"},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.PrintPdfAsImageDefault{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// PrintPdfAsImageDefault tests the PrintPdfAsImageDefault policy.
func PrintPdfAsImageDefault(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve 10 seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	url := fmt.Sprintf("%s/print_pdf_as_image_default.pdf", server.URL)

	for _, param := range []struct {
		name                    string
		wantPrintAsImageChecked checked.Checked
		policy                  *policy.PrintPdfAsImageDefault
	}{
		{
			name:                    "enabled",
			wantPrintAsImageChecked: checked.True,
			policy:                  &policy.PrintPdfAsImageDefault{Val: true},
		},
		{
			name:                    "disabled",
			wantPrintAsImageChecked: checked.False,
			policy:                  &policy.PrintPdfAsImageDefault{Val: false},
		},
		{
			name:                    "unset",
			wantPrintAsImageChecked: checked.False,
			policy:                  &policy.PrintPdfAsImageDefault{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Setup browser based on the chrome type.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			conn, err := br.NewConn(ctx, url)
			if err != nil {
				s.Fatal("Failed to open url: ", err)
			}
			defer conn.Close()
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			kb, err := input.Keyboard(ctx)
			if err != nil {
				s.Fatal("Failed to get the keyboard: ", err)
			}
			defer kb.Close()

			// Connect to Test API to use it with the UI library.
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create Test API connection: ", err)
			}

			printAsImageCheckbox := nodewith.Role(role.CheckBox).Name("Print as image")

			ui := uiauto.New(tconn)
			if err := uiauto.Combine("open print preview",
				// Wait until the PDF viewer is ready and displays the content of the PDF.
				ui.WaitUntilExists(nodewith.Role(role.StaticText).Name("Hello World")),
				kb.AccelAction("Ctrl+P"),
				ui.WaitUntilExists(printAsImageCheckbox),
			)(ctx); err != nil {
				s.Fatal("Failed to open print preview: ", err)
			}
			nodeInfo, err := ui.Info(ctx, printAsImageCheckbox)
			if err != nil {
				s.Fatal("Failed to check state of 'Print as image' checkbox: ", err)
			}

			if nodeInfo.Checked != param.wantPrintAsImageChecked {
				s.Errorf("Unexpected state of the 'Print as image' checkbox: got %s; want %s", nodeInfo.Checked, param.wantPrintAsImageChecked)
			}
		})
	}
}
