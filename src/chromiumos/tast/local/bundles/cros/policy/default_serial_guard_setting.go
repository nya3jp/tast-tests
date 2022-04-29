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

func init() {
	testing.AddTest(&testing.Test{
		Func:         DefaultSerialGuardSetting,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests the behavior of the DefaultSerialGuardSetting policy by checking that it correctly configures access to the serial port selection prompt",
		Contacts: []string{
			"cmfcmf@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr: []string{
			"group:mainline",
			"informational",
		},
		Params: []testing.Param{
			{
				Fixture: fixture.ChromePolicyLoggedIn,
				Val:     browser.TypeAsh,
			}, {
				Name:              "lacros",
				Fixture:           fixture.LacrosPolicyLoggedIn,
				Val:               browser.TypeLacros,
				ExtraSoftwareDeps: []string{"lacros"},
			},
		},
		Data: []string{defaultSerialGuardTestPage},
	})
}

const defaultSerialGuardTestPage = "default_serial_guard_setting.html"

// DefaultSerialGuardSetting tests the DefaultSerialGuardSetting policy.
func DefaultSerialGuardSetting(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	httpServer := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer httpServer.Close()

	for _, param := range []struct {
		name             string
		wantSerialDialog bool
		policy           *policy.DefaultSerialGuardSetting
	}{
		{
			name:             "blocked",
			wantSerialDialog: false,
			policy:           &policy.DefaultSerialGuardSetting{Val: 2},
		},
		{
			name:             "non_blocked",
			wantSerialDialog: true,
			policy:           &policy.DefaultSerialGuardSetting{Val: 3},
		},
		{
			name:             "unset",
			wantSerialDialog: true,
			policy:           &policy.DefaultSerialGuardSetting{Stat: policy.StatusUnset},
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
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Setup browser based on the chrome type.
			br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			conn, err := br.NewConn(ctx, fmt.Sprintf("%s/%s", httpServer.URL, defaultSerialGuardTestPage))
			if err != nil {
				s.Fatal("Failed to open website: ", err)
			}
			defer conn.Close()

			// Connect to Test API to use it with the UI library.
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create Test API connection: ", err)
			}

			ui := uiauto.New(tconn)

			// Attempt to open the serial port dialog by clicking the HTML link that
			// triggers navigator.serial.requestPort(). We cannot use conn.Eval() for
			// this, because opening the serial port dialog must be triggered by a
			// user gesture for security reasons.
			if err := ui.LeftClick(nodewith.Role(role.Link).Name("requestSerialPort"))(ctx); err != nil {
				s.Fatal("Failed to request a serial port: ", err)
			}

			if param.wantSerialDialog {
				if err := ui.WaitUntilExists(nodewith.Role(role.Window).NameContaining("connect to a serial port"))(ctx); err != nil {
					s.Fatal("Serial port selection dialog did not open: ", err)
				}
			} else {
				if err := conn.WaitForExpr(ctx, "isBlocked"); err != nil {
					s.Fatal("Failed to wait for the serial port selection dialog to be blocked: ", err)
				}
			}
		})
	}
}
