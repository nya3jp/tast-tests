// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
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
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

const captureAllowedTestHTML = "audio_capture_allowed_urls.html"

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioCaptureAllowedUrls,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checking if audio capture is allowed on websites or not, depending on the value of the policy",
		Contacts: []string{
			"cj.tsai@gmail.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{captureAllowedTestHTML},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.AudioCaptureAllowedUrls{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// AudioCaptureAllowedUrls tests the AudioCaptureAllowedUrls policy.
func AudioCaptureAllowedUrls(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		s.Fatal("Failed to parse test server URL: ", err)
	}
	serverURL.Path = filepath.Join(serverURL.Path, captureAllowedTestHTML)
	url := serverURL.String()

	for _, param := range []struct {
		name                string
		expectAskPermission bool                            // expectAskPermission states whether a dialog to ask for permission should appear or not.
		value               *policy.AudioCaptureAllowedUrls // value is the value of the policy.
	}{
		{
			// Test of permission allowed.
			name:                "allow",
			expectAskPermission: false,
			value:               &policy.AudioCaptureAllowedUrls{Val: []string{url}},
		}, {
			// Test of should ask for permission.
			name:                "notAllow",
			expectAskPermission: true,
			value:               &policy.AudioCaptureAllowedUrls{Val: []string{""}},
		}, {
			// Test of policy unset.
			name:                "unset",
			expectAskPermission: true,
			value:               &policy.AudioCaptureAllowedUrls{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
			defer cancel()

			// Setup browser based on the chrome type.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			conn, err := br.NewConn(ctx, url)
			if err != nil {
				s.Fatal("Failed to open website: ", err)
			}
			defer conn.Close()
			defer conn.CloseTarget(cleanupCtx)
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_dump_"+param.name)

			ui := uiauto.New(tconn)
			if err := ui.DoDefault(nodewith.Name("Record").Role(role.Button))(ctx); err != nil {
				s.Fatal("Failed to click Record button: ", err)
			}

			permissionWindow := nodewith.HasClass("PermissionPromptBubbleView").Role(role.Window)
			if param.expectAskPermission {
				if err = uiauto.Combine("wait for dialog pop-up and ask for permission",
					ui.WaitUntilExists(permissionWindow),
					ui.WaitUntilExists(nodewith.Name("Allow").Role(role.Button)),
				)(ctx); err != nil {
					s.Fatal("Failed to complete all actions: ", err)
				}
			} else {
				if err := uiauto.Combine("verify no prompts shows and permission is granted automatically",
					// The 15 seconds duration is an arbitrary picked timeout, should be long enough to verify no prompt will appear.
					ui.EnsureGoneFor(permissionWindow, 15*time.Second),
					ui.WaitUntilExists(nodewith.Name("This page is accessing your microphone.").Role(role.Button)),
				)(ctx); err != nil {
					s.Fatal("Failed to complete all actions: ", err)
				}
			}
		})
	}
}
