// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

const indexFileName = "auto_open_allowed_for_urls_index.html"
const downloadFileName = "auto_open_allowed_for_urls_file.zip"

func init() {
	testing.AddTest(&testing.Test{
		Func:         AutoOpenAllowedForURLs,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checking if files are auto-opened depending on the value of this policy",
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
			ExtraAttr:         []string{"informational"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		Data: []string{indexFileName, downloadFileName},
	})
}

// AutoOpenAllowedForURLs tests the AutoOpenAllowedForURLs policy.
func AutoOpenAllowedForURLs(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	indexURL := fmt.Sprintf("%s/%s", server.URL, indexFileName)
	matchingURL := fmt.Sprintf("%s/%s?matching=1", server.URL, downloadFileName)

	for _, param := range []struct {
		name          string
		linkIDToClick string
		wantAutoOpen  bool
		policy        *policy.AutoOpenAllowedForURLs
	}{
		{
			name:          "allowed_matching",
			linkIDToClick: "matching",
			wantAutoOpen:  true,
			policy:        &policy.AutoOpenAllowedForURLs{Val: []string{matchingURL}},
		},
		{
			name:          "allowed_non_matching",
			linkIDToClick: "nonMatching",
			wantAutoOpen:  false,
			policy:        &policy.AutoOpenAllowedForURLs{Val: []string{matchingURL}},
		},
		{
			name:          "unset",
			linkIDToClick: "matching",
			wantAutoOpen:  true,
			policy:        &policy.AutoOpenAllowedForURLs{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name+".txt")

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{
				param.policy,
				&policy.AutoOpenFileTypes{Val: []string{"zip"}},
			}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Setup browser based on the chrome type.
			br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			// We cannot directly open the file we are trying to download via
			// `NewConn`, since it would wait for the URL to change to the URL of the
			// downloaded file. However, Chrome does not change the URL shown in the
			// addressbar when downloading a file.
			// Instead, we open an HTML file that contains links to download the file
			// and click them via `conn.Eval`.
			conn, err := br.NewConn(ctx, indexURL)
			if err != nil {
				s.Fatal("Failed to open website: ", err)
			}
			defer conn.Close()

			clickLink := fmt.Sprintf(`document.getElementById("%s").click();`, param.linkIDToClick)
			if err := conn.Eval(ctx, clickLink, nil); err != nil {
				s.Fatal(errors.Wrap(err, "failed to click download link"))
			}

			// Connect to Test API to use it with the UI library.
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create Test API connection: ", err)
			}

			ui := uiauto.New(tconn)
			fileBrowserNode := nodewith.Role(role.Window).Name("Files - Downloads").ClassName("RootView")
			if param.wantAutoOpen {
				if err := ui.WaitUntilExists(fileBrowserNode)(ctx); err != nil {
					s.Error(errors.Wrapf(err, "unexpected auto opening behavior: got %t; want %t", false, param.wantAutoOpen))
				}
			} else {
				if err := ui.EnsureGoneFor(fileBrowserNode, 5*time.Second)(ctx); err != nil {
					s.Error(errors.Wrapf(err, "unexpected auto opening behavior: got %t; want %t", true, param.wantAutoOpen))
				}
			}
		})
	}
}
