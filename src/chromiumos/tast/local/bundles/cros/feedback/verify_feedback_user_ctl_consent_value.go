// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package feedback

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"github.com/golang/protobuf/proto"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	fpb "chromiumos/tast/local/bundles/cros/feedback/proto"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/feedbackapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VerifyFeedbackUserCtlConsentValue,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify feedbackUserCtlConsent value in the report",
		Contacts: []string{
			"wangdanny@google.com",
			"zhangwenyu@google.com",
			"xiangdongkong@google.com",
			"cros-feedback-app@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name: "share_email_and_check_consent_checkbox",
			Val:  "share_email_and_check_consent_checkbox",
		}, {
			Name: "share_email_and_uncheck_consent_checkbox",
			Val:  "share_email_and_uncheck_consent_checkbox",
		}, {
			Name: "dont_share_email",
			Val:  "dont_share_email",
		}},
	})
}

// VerifyFeedbackUserCtlConsentValue verifies the feedbackUserCtlConsent value
// in the report in different conditions.
func VerifyFeedbackUserCtlConsentValue(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	s.Log("Setting up chrome")
	cr, err := chrome.New(ctx, chrome.EnableFeatures(
		"OsFeedback", "OsFeedbackSaveReportToLocalForE2ETesting"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Test API: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr,
		"ui_dump")

	// Clean up in both beginning and the end.
	cleanUp := func() {
		if err := os.RemoveAll(feedbackapp.ReportPath); err != nil {
			s.Log("Failed to remove feedback report: ", err)
		}
	}
	cleanUp()
	defer cleanUp()

	ui := uiauto.New(tconn).WithTimeout(20 * time.Second)
	configValue := s.Param().(string)

	// Launch feedback app and go to share data page.
	feedbackRootNode, err := feedbackapp.LaunchAndGoToShareDataPage(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch feedback app and navigating to share data page: ", err)
	}

	emailDropdown := nodewith.Name("Select email").Role(role.ListBox)
	dontIncludeEmailOption := nodewith.Name("anonymous user").Role(role.ListBoxOption)
	checkboxContainer := nodewith.Name("Allow Google to email you about this issue").Role(role.GenericContainer)
	consentCheckbox := nodewith.Role(role.CheckBox).Ancestor(checkboxContainer)

	// Set up configs if needed.
	if configValue == "dont_share_email" {
		if err := uiauto.Combine("choose not to include Email",
			ui.LeftClickUntil(emailDropdown, ui.WithTimeout(
				2*time.Second).WaitUntilExists(dontIncludeEmailOption)),
			ui.LeftClick(dontIncludeEmailOption),
		)(ctx); err != nil {
			s.Fatal("Failed to choose not include Email: ", err)
		}
	} else if configValue == "share_email_and_check_consent_checkbox" {
		if err := uiauto.Combine("share email and check consent checkbox",
			ui.DoDefault(consentCheckbox),
			ui.WaitUntilExists(consentCheckbox.Attribute("checked", "true")),
		)(ctx); err != nil {
			s.Fatal("Failed to share email and check consent checkbox: ", err)
		}
	}

	// Submit the feedback and verify confirmation page title exists.
	sendButton := nodewith.Name("Send").Role(role.Button).Ancestor(feedbackRootNode)
	confirmationPageTitle := nodewith.Name("Thanks for your feedback").Role(
		role.StaticText).Ancestor(feedbackRootNode)

	if err := uiauto.Combine("Submit feedback and verify",
		ui.DoDefault(sendButton),
		ui.WaitUntilExists(confirmationPageTitle),
	)(ctx); err != nil {
		s.Fatal("Failed to submit feedback and verify: ", err)
	}

	// Read feedback report content.
	var content []byte

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		content, err = ioutil.ReadFile(feedbackapp.ReportPath)
		if err != nil {
			return errors.Wrap(err, "failed to read report content")
		}

		return nil
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		s.Fatal("Failed to read report content: ", err)
	}

	report := &fpb.ExtensionSubmit{}
	if err = proto.Unmarshal(content, report); err != nil {
		s.Fatal("Failed to parse report: ", err)
	}

	// Loop through the report array to get the feedbackUserCtlConsent value.
	feedbackUserCtlConsentValue := ""
	reportArray := report.GetWebData().GetProductSpecificData()
	for _, element := range reportArray {
		if element.GetKey() != "feedbackUserCtlConsent" {
			continue
		} else {
			feedbackUserCtlConsentValue = element.GetValue()
			break
		}
	}

	// Verify feedbackUserCtlConsent value in the feedback report.
	if configValue == "dont_share_email" || configValue == "share_email_and_uncheck_consent_checkbox" {
		if feedbackUserCtlConsentValue != "false" {
			s.Fatalf("Expected feedbackUserCtlConsent: %s does not exist", feedbackUserCtlConsentValue)
		}
	} else if configValue == "share_email_and_check_consent_checkbox" {
		if feedbackUserCtlConsentValue != "true" {
			s.Fatalf("Expected feedbackUserCtlConsent: %s does not exist", feedbackUserCtlConsentValue)
		}
	} else {
		s.Fatal("Expected feedbackUserCtlConsent does not exist")
	}
}
