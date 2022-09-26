// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/https"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

const (
	autofillCreditCardCertFile   = "certificate.pem"
	autofillCreditCardKeyFile    = "key.pem"
	autofillCreditCardCaCertFile = "ca-cert.pem"
	autofillCreditCardHTMLFile   = "autofill_credit_card_enabled.html"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AutofillCreditCardEnabled,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of AutofillCreditCardEnabled policy, checking the correspoding toggle button states (restriction and checked) after setting the policy",
		Contacts: []string{
			"chrome-autofill@google.com", // Feature owner
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		Data: []string{autofillCreditCardHTMLFile, autofillCreditCardKeyFile, autofillCreditCardCertFile, autofillCreditCardCaCertFile},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.AutofillCreditCardEnabled{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func newLocalHTTPSTestServer(htmlFile, certFile, keyFile string) (*httptest.Server, error) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			http.ServeFile(w, r, htmlFile)
		case "POST":
			fmt.Fprintf(w, "Thanks for filling in your credit card details.")
		}
	}))

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	server.StartTLS()
	return server, nil
}

func AutofillCreditCardEnabled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve 10 seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	server, err := newLocalHTTPSTestServer(s.DataPath(autofillCreditCardHTMLFile), s.DataPath(autofillCreditCardCertFile), s.DataPath(autofillCreditCardKeyFile))
	if err != nil {
		s.Fatal("Failed to start the server: ", err)
	}
	defer server.Close()

	creditCardFields := []struct {
		// The value which is set on the settings page and which should have been filled into the html input field after the autofill has been triggered.
		fieldValue string
		// The field's corresponding id on autofill_credit_card_enabled.html
		htmlFieldID string
	}{
		{
			fieldValue:  "4111111111111111", // A fake Visa fulfilling the Luhn algorithm.
			htmlFieldID: "cc-number",
		},
		{
			fieldValue:  "Tester",
			htmlFieldID: "cc-name",
		},
		{
			fieldValue:  time.Now().AddDate( /*year=*/ 1, 0, 0).Format("01/2006"),
			htmlFieldID: "cc-exp",
		},
	}

	for _, param := range []struct {
		name            string
		wantRestriction restriction.Restriction
		wantChecked     checked.Checked
		policy          *policy.AutofillCreditCardEnabled
	}{
		{
			name:            "unset",
			wantRestriction: restriction.None,
			wantChecked:     checked.True,
			policy:          &policy.AutofillCreditCardEnabled{Stat: policy.StatusUnset},
		},
		{
			name:            "allow",
			wantRestriction: restriction.None,
			wantChecked:     checked.True,
			policy:          &policy.AutofillCreditCardEnabled{Val: true},
		},
		{
			name:            "deny",
			wantRestriction: restriction.Disabled,
			wantChecked:     checked.False,
			policy:          &policy.AutofillCreditCardEnabled{Val: false},
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

			// Ensure saving payment methods toggle is accordingly enabled/disabled.
			if err := policyutil.SettingsPage(ctx, cr, br, "payments").
				SelectNode(ctx, nodewith.
					Name("Save and fill payment methods").
					Role(role.ToggleButton)).
				Restriction(param.wantRestriction).
				Checked(param.wantChecked).
				Verify(); err != nil {
				s.Error("Unexpected settings state: ", err)
			}

			// If autofilling credit card details policy is enabled, let's ensure that we can add
			// a credit card in the settings and autofill it into a credit card form.
			if param.wantChecked == checked.True {
				ui := uiauto.New(tconn)
				visaNode := nodewith.NameContaining("Visa").NameContaining("1111").Role(role.StaticText)

				isSavedAlready, err := ui.IsNodeFound(ctx, visaNode)
				if err != nil {
					s.Fatal("Failed to check if credit card node is already found: ", err)
				}

				// TODO(crbug.com/1298550): Don't rely on all files being in same directory.
				baseDirectory, _ := filepath.Split(s.DataPath(autofillCreditCardCertFile))
				serverConfiguration := https.ServerConfiguration{
					ServerKeyPath:         s.DataPath(autofillCreditCardKeyFile),
					ServerCertificatePath: s.DataPath(autofillCreditCardCertFile),
					CaCertificatePath:     s.DataPath(autofillCreditCardCaCertFile),
					HostedFilesBasePath:   baseDirectory,
				}

				// Save the certificate in chrome's certificate settings.
				if err := https.ConfigureChromeToAcceptCertificate(ctx, serverConfiguration, cr, br, tconn); err != nil {
					s.Fatal("Failed to set the certificate in Chrome's settings: ", err)
				}

				// The certificate doesn't work in 127.0.0.1 and it needs to be replaced with localhost.
				port := server.Listener.Addr().(*net.TCPAddr).Port
				urlToOpen := fmt.Sprintf("https://localhost:%d/%v", port, autofillCreditCardHTMLFile)

				// Open the website with the credit card form.
				conn, err := openCreditCardPage(ctx, br, tconn, urlToOpen)
				if err != nil {
					s.Fatal("Failed to open website: ", err)
				}
				defer conn.Close()

				// If the credit card has already been saved, saving the same card again is not possible.
				if !isSavedAlready {
					kb, err := input.Keyboard(ctx)
					if err != nil {
						s.Fatal("Failed to use keyboard: ", err)
					}
					defer kb.Close()

					// Fill in the credit card details and click on the save button.
					jsScript := "(htmlFieldID, fieldValue) => { document.getElementById(htmlFieldID).value = fieldValue; }"
					for _, creditCardField := range creditCardFields {
						if err := conn.Call(ctx, nil, jsScript, creditCardField.htmlFieldID, creditCardField.fieldValue); err != nil {
							s.Fatal("Failed to set the field value: ", err)
						}
					}

					if err := uiauto.Combine("trigger and handle the save prompt for the credit card",
						ui.DoDefaultUntil(nodewith.Name("OK").Role(role.Button).ClassName("test-target-button"), ui.Exists(visaNode)),
						ui.LeftClickUntil(nodewith.Role(role.Button).Name("Save").ClassName("MdTextButton"), ui.Exists(nodewith.NameContaining("Card saved").Role(role.StaticText))),
					)(ctx); err != nil {
						s.Fatal("Failed to save credit card: ", err)
					}

					// Re-open the website with the credit card form.
					conn.Close()
					conn, err = openCreditCardPage(ctx, br, tconn, urlToOpen)
					if err != nil {
						s.Fatal("Failed to open website: ", err)
					}
					defer conn.Close()
				}

				// Trigger the autofill on the credit card form page.
				autofillPopup := nodewith.Role(role.ListBoxOption).ClassName("AutofillPopupSuggestionView")
				if err := uiauto.Combine("clicking the Name on card field and choosing the suggested credit card",
					ui.DoDefaultUntil(nodewith.Role(role.InlineTextBox).Name("Name on card"), ui.Exists(autofillPopup)),
					ui.DoDefaultUntil(autofillPopup, ui.Exists(nodewith.Role(role.InlineTextBox).Name(creditCardFields[0].fieldValue))),
				)(ctx); err != nil {
					s.Fatal("Failed to trigger and use credit card autofill: ", err)
				}

				// Run JavaScript checks to confirm that all the fields are set correctly.
				for _, creditCardField := range creditCardFields {
					var valueFromHTML string
					if err := conn.Eval(ctx, "document.getElementById('"+creditCardField.htmlFieldID+"').value", &valueFromHTML); err != nil {
						s.Fatalf("Failed to get htmlFieldID=%s: %v", creditCardField.htmlFieldID, err)
					}
					if valueFromHTML != creditCardField.fieldValue {
						s.Errorf("Credit card field was not set properly; got %q, want %q", valueFromHTML, creditCardField.fieldValue)
					}
				}
			}
		})
	}
}

func openCreditCardPage(ctx context.Context, br *browser.Browser, tconn *chrome.TestConn, urlToOpen string) (*chrome.Conn, error) {
	ui := uiauto.New(tconn)
	conn, err := br.NewConn(ctx, urlToOpen)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open credit card page")
	}

	// Ensure the page is open.
	if err := ui.WaitUntilExists(nodewith.Name("OK").Role(role.Button).ClassName("test-target-button"))(ctx); err != nil {
		return nil, errors.Wrap(err, "expected to find the OK button on the credit card page")
	}

	return conn, nil
}
