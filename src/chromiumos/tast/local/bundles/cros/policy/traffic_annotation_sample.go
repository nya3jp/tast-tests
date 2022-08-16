// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
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
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
	//"chromiumos/tast/local/bundles/cros/policy/annotations"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TrafficAnnotationSample,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "This test is a sample test for checking traffic annotations. It doesn't verify that the policy works, it just checks for the correct logs. Proof of concept, to be modified later",
		Contacts: []string{
			"rzakarian@google.com",
			"ramyagopalan@google.com",
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
			ExtraHardwareDeps: hwdep.D(hwdep.Model("atlas")), // Enough to run on fone device.
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		Data: []string{"autofill_address_enabled.html"},
	})
}

func TrafficAnnotationSample(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve 10 seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	//tconn, err := cr.TestAPIConn(ctx)
	//if err != nil {
	//	s.Fatal("Failed to create Test API connection: ", err)
	//}

	for _, param := range []struct {
		name            string
		wantRestriction restriction.Restriction
		wantChecked     checked.Checked
		policy          *policy.AutofillAddressEnabled
	}{
		{
			name:            "allow",
			wantRestriction: restriction.None,
			wantChecked:     checked.True,
			policy:          &policy.AutofillAddressEnabled{Val: true},
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

			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			/*
				// Open the net-export page.
				netConn, err := br.NewConn(ctx, "chrome://net-export")
				if err != nil {
					s.Fatal("Failed to load chrome://net-export: ", err)
				}
				defer netConn.Close()
				// Click Start Log button.
				startLoggingBtn := `document.getElementById("start-logging")`
				if err := netConn.WaitForExpr(ctx, startLoggingBtn); err != nil {
					s.Fatal("Failed to wait for the Start Logging button to load: ", err)

				}
				if err := netConn.Eval(ctx, startLoggingBtn+`.click()`, nil); err != nil {
					s.Fatal("Failed to click the Start Logging button: ", err)
				}

				// Click Save button to choose the filename for log file.
				ui := uiauto.New(tconn)
				saveButton := nodewith.Name("Save").Role(role.Button)
				if err := uiauto.Combine("Click 'Save' button",
					ui.WaitUntilExists(saveButton),
					ui.WaitUntilEnabled(saveButton),
					ui.DoDefault(saveButton),
					ui.WaitUntilGone(saveButton),
				)(ctx); err != nil {
					s.Fatal("Failed to click: ", err)
				}
			*/
			if err := annotations.startLogging(ctx, cr, br); err != nil {
				s.Fatal("Failed to start logging: ", err)
			}
			// Open the website with the address form.
			conn, err := br.NewConn(ctx, server.URL+"/"+"autofill_address_enabled.html")
			if err != nil {
				s.Fatal("Failed to open website: ", err)
			}
			defer conn.Close()

			/*
				// Click Stop Logging button.
				stopLoggingBtn := `document.getElementById("stop-logging")`
				if err := netConn.WaitForExpr(ctx, stopLoggingBtn); err != nil {
					s.Fatal("Failed to wait for the Start Logging button to load: ", err)
				}
				if err := netConn.Eval(ctx, stopLoggingBtn+`.click()`, nil); err != nil {
					s.Fatal("Failed to click the Start Logging button: ", err)
				}

				// Get the net export log file.
				downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
				if err != nil {
					s.Fatal("Failed to get user's Download path: ", err)
				}
				downloadName := "chrome-net-export-log.json"
				downloadLocation := filepath.Join(downloadsPath, downloadName)

				// Read the net export log file.
				logFile, err := ioutil.ReadFile(downloadLocation)
				if err != nil {
					s.Fatal("Failed to open logfile: ", err)
				}
				// Check if the traffic annotation exists in the log file.
				// Specifically checking for autofill_query:88863520.
				isExist, err := regexp.Match("\"traffic_annotation\":88863520", logFile)
				if err != nil {
					s.Fatal("Failed to locate traffic annotation in logfile: ", err)
				}
				s.Log("Checking for annotation in log file, result:", isExist)
			*/
			if err := annotations.stopLoggingCheckLogs(ctx, cr, br); err != nil {
				s.Fatal("Failed to stop logging and check logs: ", err)
			}
		})
	}
}
