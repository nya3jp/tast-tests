// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

type imagesSettingTestTable struct {
	name        string          // name is the subtest name.
	wantAllowed bool            // wantAllowed is the allow state of images.
	policies    []policy.Policy // policies is a list of DefaultImagesSetting, ImagesAllowedForUrls and ImagesBlockedForUrls policies to update before checking images on URL.
}

// TODO(crbug.com/1125571): investigate using an easier filter like "*" in the allow/deny-listing policies along with DefaultImagesSetting policy.
const filterImagesURL = "http://*/images_for_url_check_index.html"
const defaultImagesSettingAllowed = 1
const defaultImagesSettingBlocked = 2

func init() {
	testing.AddTest(&testing.Test{
		Func: ImagesForURLCheck,
		Desc: "Checks the behavior of images on URL with DefaultImagesSetting, ImagesAllowedForUrls and ImagesBlockedForUrls user policies",
		Contacts: []string{
			"snijhara@google.com", // Test author
			"mohamedaomar@google.com",
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Fixture:      "chromePolicyLoggedIn",
		Data:         []string{"images_for_url_check_index.html", "images_for_url_check_index_img.jpg"},
		Params: []testing.Param{
			{
				Name: "default",
				Val: []imagesSettingTestTable{
					{
						name:        "allow",
						wantAllowed: true,
						policies:    []policy.Policy{&policy.DefaultImagesSetting{Val: defaultImagesSettingAllowed}},
					},
					{
						name:        "block",
						wantAllowed: false,
						policies:    []policy.Policy{&policy.DefaultImagesSetting{Val: defaultImagesSettingBlocked}},
					},
					{
						name:        "unset",
						wantAllowed: true,
						policies:    []policy.Policy{&policy.DefaultImagesSetting{Stat: policy.StatusUnset}},
					},
				},
			},
			{
				Name: "allowlist",
				Val: []imagesSettingTestTable{
					{
						name:        "blocklist_unset_default_block",
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
				Name: "blocklist",
				Val: []imagesSettingTestTable{
					{
						name:        "allowlist_unset_default_allow",
						wantAllowed: false,
						policies: []policy.Policy{
							&policy.ImagesBlockedForUrls{Val: []string{filterImagesURL}},
							&policy.ImagesAllowedForUrls{Stat: policy.StatusUnset},
							&policy.DefaultImagesSetting{Val: defaultImagesSettingAllowed},
						},
					},
					{
						name:        "allowlist_identical_default_allow",
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
	})
}

func ImagesForURLCheck(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fdms := s.FixtValue().(*fixtures.FixtData).FakeDMS
	tcs := s.Param().([]imagesSettingTestTable)

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

			conn, err := cr.NewConn(ctx, server.URL+"/images_for_url_check_index.html")
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
