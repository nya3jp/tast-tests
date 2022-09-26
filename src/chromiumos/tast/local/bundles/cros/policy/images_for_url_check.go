// Copyright 2020 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

type imagesSettingTestTable struct {
	name        string          // name is the subtest name.
	browserType browser.Type    // browser type used in the subtest.
	wantAllowed bool            // wantAllowed is the allow state of images.
	policies    []policy.Policy // policies is a list of DefaultImagesSetting, ImagesAllowedForUrls and ImagesBlockedForUrls policies to update before checking images on URL.
}

// TODO(crbug.com/1125571): investigate using an easier filter like "*" in the allow/deny-listing policies along with DefaultImagesSetting policy.
const filterImagesURL = "http://*/images_for_url_check_index.html"
const defaultImagesSettingAllowed = 1
const defaultImagesSettingBlocked = 2

func init() {
	testing.AddTest(&testing.Test{
		Func:         ImagesForURLCheck,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks the behavior of images on URL with DefaultImagesSetting, ImagesAllowedForUrls and ImagesBlockedForUrls user policies",
		Contacts: []string{
			"snijhara@google.com", // Test author
			"mohamedaomar@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Data:         []string{"images_for_url_check_index.html", "images_for_url_check_index_img.jpg"},
		Params: []testing.Param{
			{
				Name:    "default",
				Fixture: fixture.ChromePolicyLoggedIn,
				Val: []imagesSettingTestTable{
					{
						name:        "allow",
						browserType: browser.TypeAsh,
						wantAllowed: true,
						policies:    []policy.Policy{&policy.DefaultImagesSetting{Val: defaultImagesSettingAllowed}},
					},
					{
						name:        "block",
						browserType: browser.TypeAsh,
						wantAllowed: false,
						policies:    []policy.Policy{&policy.DefaultImagesSetting{Val: defaultImagesSettingBlocked}},
					},
					{
						name:        "unset",
						browserType: browser.TypeAsh,
						wantAllowed: true,
						policies:    []policy.Policy{&policy.DefaultImagesSetting{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name:    "allowlist",
				Fixture: fixture.ChromePolicyLoggedIn,
				Val: []imagesSettingTestTable{
					{
						name:        "blocklist_unset_default_block",
						browserType: browser.TypeAsh,
						wantAllowed: true,
						policies: []policy.Policy{
							&policy.ImagesBlockedForUrls{Stat: policy.StatusUnset},
							&policy.ImagesAllowedForUrls{Val: []string{filterImagesURL}},
							&policy.DefaultImagesSetting{Val: defaultImagesSettingBlocked},
						},
					},
				},
			},
			{
				Name:    "blocklist",
				Fixture: fixture.ChromePolicyLoggedIn,
				Val: []imagesSettingTestTable{
					{
						name:        "allowlist_unset_default_allow",
						browserType: browser.TypeAsh,
						wantAllowed: false,
						policies: []policy.Policy{
							&policy.ImagesBlockedForUrls{Val: []string{filterImagesURL}},
							&policy.ImagesAllowedForUrls{Stat: policy.StatusUnset},
							&policy.DefaultImagesSetting{Val: defaultImagesSettingAllowed},
						},
					},
					{
						name:        "allowlist_identical_default_allow",
						browserType: browser.TypeAsh,
						wantAllowed: false,
						policies: []policy.Policy{
							&policy.ImagesBlockedForUrls{Val: []string{filterImagesURL}},
							&policy.ImagesAllowedForUrls{Val: []string{filterImagesURL}},
							&policy.DefaultImagesSetting{Val: defaultImagesSettingAllowed},
						},
					},
				},
			},

			{
				Name:              "lacros_default",
				ExtraSoftwareDeps: []string{"lacros"},
				ExtraAttr:         []string{"informational"},
				Fixture:           fixture.LacrosPolicyLoggedIn,
				Val: []imagesSettingTestTable{
					{
						name:        "allow",
						browserType: browser.TypeLacros,
						wantAllowed: true,
						policies:    []policy.Policy{&policy.DefaultImagesSetting{Val: defaultImagesSettingAllowed}},
					},
					{
						name:        "block",
						browserType: browser.TypeLacros,
						wantAllowed: false,
						policies:    []policy.Policy{&policy.DefaultImagesSetting{Val: defaultImagesSettingBlocked}},
					},
					{
						name:        "unset",
						browserType: browser.TypeLacros,
						wantAllowed: true,
						policies:    []policy.Policy{&policy.DefaultImagesSetting{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name:              "lacros_allowlist",
				ExtraSoftwareDeps: []string{"lacros"},
				ExtraAttr:         []string{"informational"},
				Fixture:           fixture.LacrosPolicyLoggedIn,
				Val: []imagesSettingTestTable{
					{
						name:        "blocklist_unset_default_block",
						browserType: browser.TypeLacros,
						wantAllowed: true,
						policies: []policy.Policy{
							&policy.ImagesBlockedForUrls{Stat: policy.StatusUnset},
							&policy.ImagesAllowedForUrls{Val: []string{filterImagesURL}},
							&policy.DefaultImagesSetting{Val: defaultImagesSettingBlocked},
						},
					},
				},
			},
			{
				Name:              "lacros_blocklist",
				ExtraSoftwareDeps: []string{"lacros"},
				ExtraAttr:         []string{"informational"},
				Fixture:           fixture.LacrosPolicyLoggedIn,
				Val: []imagesSettingTestTable{
					{
						name:        "allowlist_unset_default_allow",
						browserType: browser.TypeLacros,
						wantAllowed: false,
						policies: []policy.Policy{
							&policy.ImagesBlockedForUrls{Val: []string{filterImagesURL}},
							&policy.ImagesAllowedForUrls{Stat: policy.StatusUnset},
							&policy.DefaultImagesSetting{Val: defaultImagesSettingAllowed},
						},
					},
					{
						name:        "allowlist_identical_default_allow",
						browserType: browser.TypeLacros,
						wantAllowed: false,
						policies: []policy.Policy{
							&policy.ImagesBlockedForUrls{Val: []string{filterImagesURL}},
							&policy.ImagesAllowedForUrls{Val: []string{filterImagesURL}},
							&policy.DefaultImagesSetting{Val: defaultImagesSettingAllowed},
						},
					},
				},
			},
		},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.ImagesAllowedForUrls{}, pci.VerifiedFunctionalityJS),
			pci.SearchFlag(&policy.ImagesBlockedForUrls{}, pci.VerifiedFunctionalityJS),
			pci.SearchFlag(&policy.DefaultImagesSetting{}, pci.VerifiedFunctionalityJS),
		},
	})
}

func ImagesForURLCheck(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	tcs := s.Param().([]imagesSettingTestTable)

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	for _, tc := range tcs {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, tc.policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Setup browser based on the chrome type.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, tc.browserType)
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			conn, err := br.NewConn(ctx, server.URL+"/images_for_url_check_index.html")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			shortCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer cancel()
			// Wait until the page is loaded.
			if err := conn.WaitForExpr(shortCtx, "pageReady"); err != nil {
				s.Fatal("Failed to load the page: ", err)
			}

			var imgwdth int
			if err := conn.Eval(ctx, `document.getElementById('img_id').naturalWidth`, &imgwdth); err != nil {
				s.Fatal("Failed to execute JS expression: ", err)
			}

			// If imgwdth is > 0 the image was allowed.
			if allowed := imgwdth > 0; allowed != tc.wantAllowed {
				s.Fatalf("Failed to verify images allowed behavior: got %t; want %t", allowed, tc.wantAllowed)
			}
		})
	}
}
