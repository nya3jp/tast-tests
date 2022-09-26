// Copyright 2022 The ChromiumOS Authors
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
		Func:         AutofillAddressEnabled,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of AutofillAddressEnabled policy, checking the correspoding toggle button states (restriction and checked) after setting the policy",
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
		Data: []string{"autofill_address_enabled.html"},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.AutofillAddressEnabled{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func AutofillAddressEnabled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve 10 seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	addressValues := []struct {
		// The field's name on the settings page.
		fieldName string
		// The value which is set on the settings page and which should have been filled into the html input field after the autofill has been triggered.
		fieldValue string
		// The field's corresponding id on autofill_address_enabled.html
		htmlFieldID string
	}{
		{
			fieldName:   "Name",
			fieldValue:  "Tester",
			htmlFieldID: "name",
		},
		{
			fieldName:   "Street address",
			fieldValue:  "Some address 123",
			htmlFieldID: "street-address",
		},
		{
			fieldName:   "City",
			fieldValue:  "City",
			htmlFieldID: "city",
		},
		{
			fieldName:   "ZIP code",
			fieldValue:  "11111",
			htmlFieldID: "postal-code",
		},
		{
			fieldName:   "Phone",
			fieldValue:  "0441231234",
			htmlFieldID: "phone",
		},
		{
			fieldName:   "Email",
			fieldValue:  "test@gmail.com",
			htmlFieldID: "email",
		},
	}

	for _, param := range []struct {
		name            string
		wantRestriction restriction.Restriction
		wantChecked     checked.Checked
		policy          *policy.AutofillAddressEnabled
	}{
		{
			name:            "unset",
			wantRestriction: restriction.None,
			wantChecked:     checked.True,
			policy:          &policy.AutofillAddressEnabled{Stat: policy.StatusUnset},
		},
		{
			name:            "allow",
			wantRestriction: restriction.None,
			wantChecked:     checked.True,
			policy:          &policy.AutofillAddressEnabled{Val: true},
		},
		{
			name:            "deny",
			wantRestriction: restriction.Disabled,
			wantChecked:     checked.False,
			policy:          &policy.AutofillAddressEnabled{Val: false},
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

			if err := policyutil.SettingsPage(ctx, cr, br, "addresses").
				SelectNode(ctx, nodewith.
					Name("Save and fill addresses").
					Role(role.ToggleButton)).
				Restriction(param.wantRestriction).
				Checked(param.wantChecked).
				Verify(); err != nil {
				s.Error("Unexpected settings state: ", err)
			}

			if param.wantChecked == checked.True {
				ui := uiauto.New(tconn)

				if err := uiauto.Combine("open the add address dialog",
					ui.DoDefault(nodewith.Name("Add address").Role(role.Button)),
					ui.WaitUntilExists(nodewith.Name("Save").Role(role.Button)),
				)(ctx); err != nil {
					s.Fatal("Failed to open the add address dialog: ", err)
				}

				kb, err := input.Keyboard(ctx)
				if err != nil {
					s.Fatal(errors.Wrap(err, "failed to get the keyboard"))
				}
				defer kb.Close()

				// Fill in the address input fields and click on the save button.
				for _, address := range addressValues {
					addressField := nodewith.Role(role.TextField).Name(address.fieldName)
					if err := uiauto.Combine("fill in address text field",
						ui.MakeVisible(addressField),
						ui.FocusAndWait(addressField),
					)(ctx); err != nil {
						s.Fatal("Failed to click the text field: ", err)
					}
					if err := kb.Type(ctx, address.fieldValue); err != nil {
						s.Fatal("Failed to type to the text field: ", err)
					}
				}
				if err := ui.DoDefault(nodewith.Role(role.Button).Name("Save"))(ctx); err != nil {
					s.Fatal("Failed to click the Save button: ", err)
				}

				// Open the website with the address form.
				conn, err := br.NewConn(ctx, server.URL+"/"+"autofill_address_enabled.html")
				if err != nil {
					s.Fatal("Failed to open website: ", err)
				}
				defer conn.Close()

				// Trigger the autofill by clicking the email field and choosing the suggested address (this could be any of the address fields).
				suggestionPopup := nodewith.Role(role.ListBoxOption).ClassName("AutofillPopupSuggestionView")
				emailTextBox := nodewith.Role(role.InlineTextBox).Name("Email")
				if err := uiauto.Combine("clicking the Email field and choosing the suggested address",
					ui.WaitUntilExists(nodewith.Name("OK").Role(role.Button).ClassName("test-target-button")),
					ui.MakeVisible(emailTextBox),
					ui.DoDefault(emailTextBox),
					ui.WithTimeout(45*time.Second).WaitUntilExists(suggestionPopup),
					ui.DoDefaultUntil(suggestionPopup, ui.Exists(nodewith.Role(role.InlineTextBox).Name(addressValues[1].fieldValue))),
				)(ctx); err != nil {
					s.Fatal("Failed to trigger and use address autofill: ", err)
				}

				// Run JavaScript checks to confirm that all the address fields are set correctly.
				for _, address := range addressValues {
					var valueFromHTML string
					if err := conn.Eval(ctx, "document.getElementById('"+address.htmlFieldID+"').value", &valueFromHTML); err != nil {
						s.Fatal("Failed to complete the JS test for htmlFieldID="+address.htmlFieldID, err)
					}
					if valueFromHTML != address.fieldValue {
						s.Fatal("Address was not set properly. Actual value " + valueFromHTML + " doesnt match with expected " + address.fieldValue)
					}
				}
			}
		})
	}
}
