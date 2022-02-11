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
	"path"
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

const autofillCreditCardCertFile = "autofill_credit_card_enabled_certificate.pem"
const autofillCreditCardKeyFile = "autofill_credit_card_enabled_key.pem"
const autofillCreditCardCaCertFile = "autofill_credit_card_enabled_cacert.pem"
const autofillCreditCardCaCertName = "TastCA"
const autofillCreditCardCaCertAuthName = "org-Google"
const autofillCreditCardHTMLFile = "autofill_credit_card_enabled.html"

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

func newLocalHTTPSTestServer(handler http.Handler, certFile, keyFile string) (*httptest.Server, error) {
	server := httptest.NewUnstartedServer(handler)
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

	server, err := newLocalHTTPSTestServer(http.FileServer(s.DataFileSystem()), s.DataPath(autofillCreditCardCertFile), s.DataPath(autofillCreditCardKeyFile))
	if err != nil {
		s.Fatal("Failed to start the server: ", err)
	}
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
					s.Fatal("Failed to get the keyb: ", err)
				}
				defer kb.Close()

				// Fill in the credit card details and click on the save button.
				// TODO(crbug.com/1294166): Do the clicking on the test page rather than on the settings page.
				for _, creditCardField := range creditCardFields {
					textField := nodewith.Role(role.TextField).Name(creditCardField.fieldName)
					if err := uiauto.Combine("fill in the credit card details",
						ui.MakeVisible(textField),
						ui.LeftClick(textField),
						kb.TypeAction(creditCardField.fieldValue),
					)(ctx); err != nil {
						s.Fatal(errors.Wrap(err, "failed to fill in the credit card details"))
					}
				}

				if err := ui.LeftClick(nodewith.Role(role.Button).Name("Save"))(ctx); err != nil {
					s.Fatal("Failed to click the Save button: ", err)
				}

				// Save the certificate in chrome's certificate settings.
				if err := configureChromeToAcceptCertificate(ctx, cr, br, tconn, s.DataPath(autofillCreditCardCaCertFile)); err != nil {
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

// checkIfCertificateExists checks whether the certificate that is tried to add already exists.
func checkIfCertificateExists(ctx context.Context, cr *chrome.Chrome, br *browser.Browser, tconn *chrome.TestConn, certName, authOrgName string) (bool, error) {
	ui := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return false, err
	}
	defer kb.Close()

	policyutil.SettingsPage(ctx, cr, br, "certificates")
	authorities := nodewith.Name("Authorities").Role(role.Tab)
	authTabText := nodewith.Name("You have certificates on file that identify these certificate authorities").Role(role.StaticText)

	if err := uiauto.Combine("open correct tab",
		ui.WaitUntilExists(authorities),
		ui.LeftClick(authorities),
		ui.WaitUntilExists(authTabText),
	)(ctx); err != nil {
		return false, err
	}

	googleCertCollapsiblePanel := nodewith.Name(authOrgName).Role(role.InlineTextBox)

	// Authority of the cert doesn't exist, so cert also cannot exist.
	if err := ui.Exists(googleCertCollapsiblePanel)(ctx); err != nil {
		return false, nil
	}

	if err := uiauto.Combine("open collapsible panel to check if the cert is there",
		ui.MakeVisible(googleCertCollapsiblePanel),
		ui.LeftClick(googleCertCollapsiblePanel),
		kb.AccelAction("Tab"),
		kb.AccelAction("Enter"),
	)(ctx); err != nil {
		return false, err
	}

	certificateText := nodewith.Name(certName).Role(role.StaticText)
	// The actual cert doesn't exist.
	if err := ui.Exists(certificateText)(ctx); err != nil {
		return false, nil
	}

	return true, nil
}

// configureChromeToAcceptCertificate sets Chrome's certificate settings.
// TODO(crbug.com/1293286): Re-use isolatedapp.ConfigureChromeToAcceptCertificate() instead when it has been moved to a directory which is accessible from here.
func configureChromeToAcceptCertificate(ctx context.Context, cr *chrome.Chrome, br *browser.Browser, tconn *chrome.TestConn, caCertDataPath string) error {
	// Check if the cert already exists -> No need to add again.
	if certExists, err := checkIfCertificateExists(ctx, cr, br, tconn, autofillCreditCardCaCertName, autofillCreditCardCaCertAuthName); err != nil {
		return err
	} else if certExists {
		return nil
	}

	// Copy the certificate file to a local Downloads directory.
	bytesRead, err := ioutil.ReadFile(caCertDataPath)
	if err != nil {
		return err
	}
	ioutil.WriteFile(path.Join(filesapp.MyFilesPath, filesapp.Downloads, autofillCreditCardCaCertFile), bytesRead, 0644)

	// Add the certificate in the certificate settings.
	policyutil.SettingsPage(ctx, cr, br, "certificates")
	authorities := nodewith.Name("Authorities").Role(role.Tab)
	authTabText := nodewith.Name("You have certificates on file that identify these certificate authorities").Role(role.StaticText)
	importButton := nodewith.Name("Import").Role(role.Button)
	certFileItem := nodewith.Name(autofillCreditCardCaCertFile).First()
	openButton := nodewith.Name("Open").Role(role.Button)
	trust1Checkbox := nodewith.Name("Trust this certificate for identifying websites").Role(role.CheckBox)
	trust2Checkbox := nodewith.Name("Trust this certificate for identifying email users").Role(role.CheckBox)
	trust3Checkbox := nodewith.Name("Trust this certificate for identifying software makers").Role(role.CheckBox)
	okButton := nodewith.Name("OK").Role(role.Button)

	ui := uiauto.New(tconn)
	if err := uiauto.Combine("set_cerficiate",
		ui.WaitUntilExists(authorities),
		ui.LeftClick(authorities),
		ui.WaitUntilExists(authTabText),
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
		return err
	}

	// Check if addition was successful.
	if certExists2, err := checkIfCertificateExists(ctx, cr, br, tconn, autofillCreditCardCaCertName, autofillCreditCardCaCertAuthName); err != nil {
		return err
	} else if !certExists2 {
		return errors.Errorf("failed to add certificate with name %q", autofillCreditCardCaCertName)
	}

	return nil
}
