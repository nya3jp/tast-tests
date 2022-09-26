// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DownloadRestrictions,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of DownloadRestrictions policy, check if a file is downloaded or not based on the value of the policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		Data: []string{"download_restrictions_index.html", "download_restrictions.zip"},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.DownloadRestrictions{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func DownloadRestrictions(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Clear Downloads directory.
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}
	files, err := ioutil.ReadDir(downloadsPath)
	if err != nil {
		s.Fatal("Failed to get files from Downloads directory")
	}
	for _, file := range files {
		if err = os.RemoveAll(filepath.Join(downloadsPath, file.Name())); err != nil {
			s.Fatal("Failed to remove file: ", file.Name())
		}
	}

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	for _, param := range []struct {
		name    string
		blocked bool
		policy  *policy.DownloadRestrictions // policy is the policy we test.
	}{
		{
			name:    "unset",
			blocked: false,
			policy:  &policy.DownloadRestrictions{Stat: policy.StatusUnset},
		},
		{
			name:    "block_downloads",
			blocked: true,
			policy:  &policy.DownloadRestrictions{Val: 3}, // 3: all downloads are blocked
		},
		{
			name:    "allow_downloads",
			blocked: false,
			policy:  &policy.DownloadRestrictions{Val: 0}, // 0: all downloads are allowed
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

			dconn, err := br.NewConn(ctx, server.URL+"/download_restrictions_index.html")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer dconn.Close()

			err = dconn.Eval(ctx, `document.getElementById('dlink').click()`, nil)
			if err != nil {
				s.Fatal("Failed to execute JS expression: ", err)
			}

			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create Test API connection: ", err)
			}

			files, err := filesapp.Launch(ctx, tconn)
			if err != nil {
				s.Fatal("Launching the Files App failed: ", err)
			}
			defer files.Close(ctx)

			if err := files.OpenDownloads()(ctx); err != nil {
				s.Fatal("Opening Downloads folder failed: ", err)
			}
			if err := files.WithTimeout(5 * time.Second).WaitForFile("download_restrictions.zip")(ctx); err != nil {
				if !param.blocked {
					if errors.Is(err, context.DeadlineExceeded) {
						s.Error("Download was blocked: ", err)
					} else {
						s.Fatal("Failed to wait for download_restrictions.zip: ", err)
					}
				}
			} else {
				if param.blocked {
					s.Error("Download was not blocked")
				}
				if err := os.Remove(filepath.Join(downloadsPath, "download_restrictions.zip")); err != nil {
					s.Error("Failed to remove download_restrictions.zip: ", err)
				}
			}
		})
	}
}
