// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"crypto/tls"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
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
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
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
		Data: []string{htmlFileName, keyFileName, certFileName, caCertFileName},
	})
}

const certFileName = "certificate.pem"
const keyFileName = "key.pem"
const caCertFileName = "ca-cert.pem"
const htmlFileName = "autofill_credit_card_enabled.html"

// func NewLocalHTTPSTestServer(handler http.Handler, certFile string, keyFile string) (*httptest.Server, error) {
// 	server := httptest.NewUnstartedServer(handler)
// 	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
// 	if err != nil {
// 		return nil, err
// 	}
// 	server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
// 	server.StartTLS()
// 	return server, nil
// }

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

	// Autofilling credit card details requires the server to use HTTPS.
	server := httptest.NewUnstartedServer(http.FileServer(s.DataFileSystem()))
	cert, err := tls.LoadX509KeyPair(s.DataPath(certFileName), s.DataPath(keyFileName))
	if err != nil {
		s.Fatal("Failed to read cert key pair: ", err)
	}
	server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	server.StartTLS()
	defer server.Close()

	creditCardFields := []struct {
		// The field's name on the Chrome's settings page.
		fieldName string
		// The value which is set on the settings page and which should have been filled into the html input field after the autofill has been triggered.
		fieldValue string
		// The field's corresponding id on autofill_credit_card_enabled.html
		htmlFieldID string
	}{
		{
			fieldName:   "Card number",
			fieldValue:  "1234123412341234",
			htmlFieldID: "cc-number",
		},
		{
			fieldName:   "Name on card",
			fieldValue:  "Tester",
			htmlFieldID: "cc-name",
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
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

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

			if err := policyutil.SettingsPage(ctx, cr, br, "payments").
				SelectNode(ctx, nodewith.
					Name("Payment methods").
					Role(role.ToggleButton)).
				Restriction(param.wantRestriction).
				Checked(param.wantChecked).
				Verify(); err != nil {
				s.Error("Unexpected settings state: ", err)
			}

			if param.wantChecked == checked.True {
				ui := uiauto.New(tconn)

				// Click the button to add a new credit card.
				if err := ui.LeftClick(nodewith.Name("Add").Role(role.Button))(ctx); err != nil {
					s.Fatal("Failed to click the Add button: ", err)
				}

				// Find the Save button node, meaning the form is open.
				if err := ui.WaitUntilExists(nodewith.Name("Save").Role(role.Button))(ctx); err != nil {
					s.Fatal("Failed to find the Save button: ", err)
				}

				kb, err := input.Keyboard(ctx)
				if err != nil {
					s.Fatal(errors.Wrap(err, "failed to get the keyboard"))
				}
				defer kb.Close()

				// Fill in the credit card details and click on the save button.
				for _, creditCardField := range creditCardFields {
					textField := nodewith.Role(role.TextField).Name(creditCardField.fieldName)
					if err := ui.MakeVisible(textField)(ctx); err != nil {
						s.Fatal("Failed to make the text field visible: ", err)
					}
					if err := ui.LeftClick(textField)(ctx); err != nil {
						s.Fatal("Failed to click the text field: ", err)
					}
					if err := kb.Type(ctx, creditCardField.fieldValue); err != nil {
						s.Fatal("Failed to type to the text field: ", err)
					}
				}

				if err := ui.LeftClick(nodewith.Role(role.Button).Name("Save"))(ctx); err != nil {
					s.Fatal("Failed to click the Save button: ", err)
				}

				// Save the certificate in chrome's certificate settings.
				// TODO(crbugs.com/1293286): Re-use isolatedapp.ConfigureChromeToAcceptCertificate() instead when it has been moved to a directory which is accessible from here.
				bytesRead, err := ioutil.ReadFile(s.DataPath(caCertFileName))
				if err != nil {
					s.Fatal("Failed to read the file: ", err)
				}
				ioutil.WriteFile(filesapp.MyFilesPath+"/"+filesapp.Downloads+"/"+caCertFileName, bytesRead, 0644)
				policyutil.SettingsPage(ctx, cr, br, "certificates")

				authorities := nodewith.Name("Authorities").Role(role.Tab)
				importButton := nodewith.Name("Import").Role(role.Button)
				certFileItem := nodewith.Name(caCertFileName).First()
				openButton := nodewith.Name("Open").Role(role.Button)
				trust1Checkbox := nodewith.Name("Trust this certificate for identifying websites").Role(role.CheckBox)
				trust2Checkbox := nodewith.Name("Trust this certificate for identifying email users").Role(role.CheckBox)
				trust3Checkbox := nodewith.Name("Trust this certificate for identifying software makers").Role(role.CheckBox)
				okButton := nodewith.Name("OK").Role(role.Button)

				if err := uiauto.Combine("set_cerficiate",
					ui.WaitUntilExists(authorities),
					ui.LeftClick(authorities),
					ui.WaitUntilExists(importButton),
					ui.LeftClick(importButton),
					ui.WaitUntilExists(certFileItem),
					ui.LeftClick(certFileItem),
					ui.WaitUntilExists(openButton),
					ui.LeftClick(openButton),
					ui.WaitUntilExists(trust1Checkbox),
					ui.LeftClick(trust1Checkbox),
					ui.LeftClick(trust2Checkbox),
					ui.LeftClick(trust3Checkbox),
					ui.LeftClick(okButton),
				)(ctx); err != nil {
					s.Fatal("Failed to save the certificate: ", err)
				}

				// Open the website with the credit card form.
				urlToOpen := strings.Replace(server.URL, "127.0.0.1", "localhost", -1) + "/" + htmlFileName
				conn, err := br.NewConn(ctx, urlToOpen)
				if err != nil {
					s.Fatal("Failed to open website: ", err)
				}
				defer conn.Close()

				if err := ui.WaitUntilExists(nodewith.Name("OK").Role(role.Button))(ctx); err != nil {
					s.Fatal("Failed to find the ok button: ", err)
				}

				// Trigger the autofill by clicking the Name on card field and choosing the suggested credit card (this could be any of the credit card fields).
				if err := ui.LeftClick(nodewith.Role(role.InlineTextBox).Name("Name on card"))(ctx); err != nil {
					s.Fatal("Failed to click the Name on card text field: ", err)
				}

				if err := ui.LeftClick(nodewith.Role(role.ListBoxOption).ClassName("AutofillPopupSuggestionView"))(ctx); err != nil {
					s.Fatal("Failed to click the AutofillPopupSuggestionView listBoxOption: ", err)
				}

				// Run JavaScript checks to confirm that all the fields are set correctly.
				for _, creditCardField := range creditCardFields {
					var valueFromHTML string
					if err := conn.Eval(ctx, "document.getElementById('"+creditCardField.htmlFieldID+"').value", &valueFromHTML); err != nil {
						s.Fatal("Failed to complete the JS test for htmlFieldID="+creditCardField.htmlFieldID, err)
					}
					if valueFromHTML != creditCardField.fieldValue {
						s.Fatal("Credit card field was not set properly. Actual value " + valueFromHTML + " does not match with expected " + creditCardField.fieldValue)
					}
				}
			}
		})
	}
}
