// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
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
			"laurila@google.com", // Test author
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
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		Data: []string{autofillCreditCardHTMLFile, autofillCreditCardKeyFile, autofillCreditCardCertFile, autofillCreditCardCaCertFile},
	})
}

func newLocalHTTPSTestServer(htmlFile, certFile, keyFile string) (*httptest.Server, error) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			http.ServeFile(w, r, htmlFile)
		case "POST":
			// TODO: What should I do here to ensure the credit card details get saved?
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
			fieldValue:  "1234123412341234",
			htmlFieldID: "cc-number",
		},
		{
			fieldValue:  "Tester",
			htmlFieldID: "cc-name",
		},
		{
			fieldValue:  time.Now().Format("2006-01"),
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
			br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			if err := policyutil.SettingsPage(ctx, cr, br, "payments").
				SelectNode(ctx, nodewith.
					Name("Payment methods").
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
				urlToOpen := strings.Replace(server.URL, "127.0.0.1", "localhost", -1) + "/" + autofillCreditCardHTMLFile

				// Open the website with the credit card form.
				conn, err := br.NewConn(ctx, urlToOpen)
				if err != nil {
					s.Fatal("Failed to open website: ", err)
				}
				defer conn.Close()

				if err := ui.WaitUntilExists(nodewith.Name("OK").Role(role.Button))(ctx); err != nil {
					s.Fatal("Failed to find the OK button: ", err)
				}

				// Fill in the credit card details and click on the save button.
				for _, creditCardField := range creditCardFields {
					if err := conn.Call(ctx, nil, `(htmlFieldID, fieldValue) => {
					  document.getElementById(htmlFieldID).value = fieldValue;
				  
					}`, creditCardField.htmlFieldID, creditCardField.fieldValue); err != nil {
						s.Fatal("Failed to change the field value: ", err)
					}
				}
				if err := ui.LeftClick(nodewith.Name("OK").Role(role.Button))(ctx); err != nil {
					s.Fatal("Failed to click the OK button: ", err)
				}

				// Re-open the website with the credit card form.
				conn, err = br.NewConn(ctx, urlToOpen)
				if err != nil {
					s.Fatal("Failed to open website: ", err)
				}
				defer conn.Close()

				if err := ui.WaitUntilExists(nodewith.Name("OK").Role(role.Button))(ctx); err != nil {
					s.Fatal("Failed to find the OK button: ", err)
				}

				// Trigger the autofill on the credit card form page.
				if err := uiauto.Combine("clicking the Name on card field and choosing the suggested credit card",
					ui.WaitUntilExists(nodewith.Name("OK").Role(role.Button)),
					ui.LeftClick(nodewith.Role(role.InlineTextBox).Name("Name on card")),
					ui.LeftClick(nodewith.Role(role.ListBoxOption).ClassName("AutofillPopupSuggestionView")),
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
