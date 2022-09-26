// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

const (
	indexFileName    = "auto_open_allowed_for_urls_index.html"
	downloadFileName = "auto_open_allowed_for_urls_file.zip"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AutoOpenAllowedForURLs,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checking if files are auto-opened depending on the value of this policy",
		Contacts: []string{
			"cmfcmf@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
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
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.AutoOpenAllowedForURLs{}, pci.VerifiedFunctionalityUI),
			pci.SearchFlag(&policy.AutoOpenFileTypes{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// AutoOpenAllowedForURLs tests the AutoOpenAllowedForURLs policy.
func AutoOpenAllowedForURLs(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

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

			// Reserve 10 seconds for cleanup.
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer cancel()

			// Setup browser based on the chrome type.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// We cannot directly open the file we are trying to download via
			// NewConn(), since NewConn() expects Chrome to navigate to the URL
			// passed to it. However, Chrome does not navigate and change its URL when
			// downloading a file. Instead, Chrome continues to show the current page.
			// To circumvent this problem, we open an HTML file that contains links to
			// download the file and click them via Eval().
			conn, err := br.NewConn(ctx, indexURL)
			if err != nil {
				s.Fatal("Failed to open website: ", err)
			}
			defer conn.Close()

			downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
			if err != nil {
				s.Fatal("Failed to get users Download path: ", err)
			}

			clickLink := fmt.Sprintf(`document.getElementById("%s").click();`, param.linkIDToClick)
			if err := conn.Eval(ctx, clickLink, nil); err != nil {
				s.Fatal("Failed to click download link: ", err)
			}
			defer os.Remove(path.Join(downloadsPath, downloadFileName))

			// Connect to Test API to use it with the UI library.
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create Test API connection: ", err)
			}

			ui := uiauto.New(tconn)
			fileBrowserNode := filesapp.WindowFinder(apps.FilesSWA.ID)
			if param.wantAutoOpen {
				if err := ui.WaitUntilExists(fileBrowserNode)(ctx); err != nil {
					s.Error("Failed to wait for file to auto open: ", err)
				}
			} else {
				if err := ui.EnsureGoneFor(fileBrowserNode, 5*time.Second)(ctx); err != nil {
					s.Error("File unexpectedly auto opened: ", err)
				}
			}
		})
	}
}
