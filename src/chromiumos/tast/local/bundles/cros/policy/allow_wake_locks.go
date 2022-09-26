// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AllowWakeLocks,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of AllowWakeLocks policy check whether it shows idle window or not for pages with wake locks requests",
		Contacts: []string{
			"mohamedaomar@google.com", // Test author
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
		Data: []string{"allow_wake_locks_index.html"},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.AllowWakeLocks{}, pci.VerifiedFunctionalityUI),
			pci.SearchFlag(&policy.PowerManagementIdleSettings{}, pci.VerifiedValue),
		},
	})
}

// AllowWakeLocks tests the AllowWakeLocks policy.
func AllowWakeLocks(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	// Set idle warning window to show after 0.5 seconds, and idle after 5 minutes.
	// IdleAction is Logout so that the idle window could be shown; DoNothing won't show the idle window.
	var powerManagementSettingsJSON = &policy.RefPowerManagementDelays{
		IdleAction: "Logout",
		Delays: &policy.RefPowerManagementDelaysDelays{
			Idle:        300000,
			IdleWarning: 500,
			ScreenDim:   300000,
			ScreenOff:   300000,
		},
	}

	var powerManagementIdleSettingsPolicy = &policy.PowerManagementIdleSettings{Val: &policy.PowerManagementIdleSettingsValue{
		AC:      powerManagementSettingsJSON,
		Battery: powerManagementSettingsJSON,
	}}

	for _, param := range []struct {
		name           string
		wantIdleWindow bool // wantIdleWindow is a flag to check if idle window should popup.
		policies       []policy.Policy
	}{
		{
			name:           "enabled",
			wantIdleWindow: false,
			policies: []policy.Policy{
				powerManagementIdleSettingsPolicy,
				&policy.AllowWakeLocks{Val: true},
			},
		},
		{
			name:           "disabled",
			wantIdleWindow: true,
			policies: []policy.Policy{
				powerManagementIdleSettingsPolicy,
				&policy.AllowWakeLocks{Val: false},
			},
		},
		{
			name:           "unset",
			wantIdleWindow: false,
			policies: []policy.Policy{
				powerManagementIdleSettingsPolicy,
				&policy.AllowWakeLocks{Stat: policy.StatusUnset},
			},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Setup browser based on the chrome type.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Open a page with wake locks request.
			conn, err := br.NewConn(ctx, server.URL+"/allow_wake_locks_index.html")
			if err != nil {
				s.Fatal("Failed to connect to chrome: ", err)
			}
			defer conn.Close()

			res := ""
			// Call the wake lock request and evaluate its response.
			if err := conn.Eval(ctx, `requestWakeLock()`, &res); err != nil {
				s.Fatal("Failed to execute JS expression: ", err)
			}
			if res != "" {
				s.Fatal("Calling wake lock returned error: ", res)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, param.policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Create a uiauto.Context with default timeout.
			ui := uiauto.New(tconn)

			idleWindow := nodewith.ClassName("IdleActionWarningDialogView").Role(role.Window)
			if param.wantIdleWindow {
				// If AllowWakeLocks is disabled then the idle window will popup.
				if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(idleWindow.First())(ctx); err != nil {
					s.Fatal("Failed to find the idle window: ", err)
				}
			} else {
				// If AllowWakeLocks is enabled/unset then the screen will remain awake and no idle window will popup.
				if err := ui.EnsureGoneFor(idleWindow, 10*time.Second)(ctx); err != nil {
					s.Fatal("Failed to make sure no idle window popup: ", err)
				}
			}
		})
	}
}
