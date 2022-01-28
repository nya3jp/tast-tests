// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
		Data: []string{"autofill_credit_card_enabled.html"},
	})
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

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
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

				// Open the website with the credit card form.
				conn, err := br.NewConn(ctx, server.URL+"/"+"autofill_credit_card_enabled.html")
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

				// TODO(laurila): Blocked due to the insecure form warning until crbug.com/1292020 has landed.
				// &policy.InsecureFormsWarningsEnabled{Val: false} also did not work to by-pass this.

				// The rows below this lines do not pass the tests yet (before HTTPS connection). Uncomment when ready.

				// if err := ui.LeftClick(nodewith.Role(role.ListBoxOption).ClassName("AutofillPopupSuggestionView"))(ctx); err != nil {
				// 	s.Fatal("Failed to click the AutofillPopupSuggestionView listBoxOption: ", err)
				// }

				// // Run JavaScript checks to confirm that all the fields are set correctly.
				// for _, creditCardField := range creditCardFields {
				// 	var valueFromHTML string
				// 	if err := conn.Eval(ctx, "document.getElementById('"+creditCardField.htmlFieldID+"').value", &valueFromHTML); err != nil {
				// 		s.Fatal("Failed to complete the JS test for htmlFieldID="+creditCardField.htmlFieldID, err)
				// 	}
				// 	if valueFromHTML != creditCardField.fieldValue {
				// 		s.Fatal("Credit card field was not set properly. Actual value " + valueFromHTML + "+ does not match with expected " + creditCardField.fieldValue)
				// 	}
				// }
			}
		})
	}
}
