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

func init() {
	testing.AddTest(&testing.Test{
		Func:         DefaultJavaScriptSetting,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Check that the DefaultJavaScript policy blocks or allows JavaScript",
		Contacts: []string{
			"mpolzer@google.com", // Test author
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
		Data: []string{"js_test.html"},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.DefaultJavaScriptSetting{}, pci.VerifiedFunctionalityJS),
		},
	})
}

func DefaultJavaScriptSetting(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	for _, tc := range []struct {
		name  string
		value *policy.DefaultJavaScriptSetting
		allow bool
	}{
		{
			name:  "allow",
			value: &policy.DefaultJavaScriptSetting{Val: 1},
			allow: true,
		},
		{
			name:  "block",
			value: &policy.DefaultJavaScriptSetting{Val: 2},
			allow: false,
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to reset Chrome: ", err)
			}

			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{tc.value}); err != nil {
				s.Fatal("Failed to serve and verify policies: ", err)
			}

			// TODO(crbug.com/1259615): This should be part of the fixture.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to setup chrome: ", err)
			}
			defer closeBrowser(cleanupCtx)

			conn, err := br.NewConn(ctx, server.URL+"/js_test.html")
			if err != nil {
				s.Fatal("Failed to connect to Chrome: ", err)
			}
			defer conn.Close()

			allowed := false
			// Evaluating is going to fail if JavaScript is not allowed.
			if err := conn.Eval(ctx, "jsAllowed", &allowed); err != nil && tc.allow {
				s.Fatal("Failed to read jsAllowed although JavaScript should be allowed: ", err)
			} else {
				if allowed != tc.allow {
					s.Fatalf("Unexpected jsAllowed value; got %v, want %v", allowed, tc.allow)
				}
			}
		})
	}
}
