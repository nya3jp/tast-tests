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
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/localstate"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/feedbackapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ReportContainsEmail,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify feedback report contains user email",
		Contacts: []string{
			"wangdanny@google.com",
			"zhangwenyu@google.com",
			"xiangdongkong@google.com",
			"cros-feedback-app@google.com",
		},
		Fixture:      "chromeLoggedInWithOsFeedbackSaveReportToLocalForE2ETesting",
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name: "include_email",
			Val:  true,
		}, {
			Name: "not_include_email",
			Val:  false,
		}},
	})
}

// ReportContainsEmail verifies feedback report contains user email.
func ReportContainsEmail(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Test API: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr,
		"ui_dump")

	// Clean up in the end.
	defer func() {
		if err := os.RemoveAll(feedbackapp.ReportPath); err != nil {
			s.Error("Failed to remove feedback report: ", err)
		}
	}()

	ui := uiauto.New(tconn).WithTimeout(20 * time.Second)
	expectedEmailInFeedback := s.Param().(bool)

	// Launch feedback app and go to share data page.
	feedbackRootNode, err := feedbackapp.LaunchAndGoToShareDataPage(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch feedback app and navigating to share data page: ", err)
	}

	// Find default selected email address.
	var localState struct {
		Emails []string `json:"LoggedInUsers"`
	}

	if err := localstate.Unmarshal(browser.TypeAsh, &localState); err != nil {
		s.Fatal("Failed to extract Local State: ", err)
	}

	var selectedEmail string
	for _, email := range localState.Emails {
		if err := ui.WaitUntilExists(nodewith.NameContaining(email).First())(
			ctx); err == nil {
			selectedEmail = email
		}
	}

	emailDropdown := nodewith.Name("Select email").Role(role.ListBox)
	dontIncludeEmailOption := nodewith.Name("anonymous user").Role(role.ListBoxOption)

	// Choose not to include email if needed.
	if !expectedEmailInFeedback {
		if err := uiauto.Combine("choose not to include Email",
			ui.LeftClickUntil(emailDropdown, ui.WithTimeout(
				2*time.Second).WaitUntilExists(dontIncludeEmailOption)),
			ui.LeftClick(dontIncludeEmailOption),
		)(ctx); err != nil {
			s.Fatal("Failed to choose not include Email: ", err)
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

	userEmail := report.GetCommonData().GetUserEmail()
	expectedEmail := selectedEmail

	// Verify feedback report contains email based on user selection.
	if expectedEmailInFeedback {
		if userEmail != selectedEmail {
			s.Fatalf("Expected email %s does not exist", expectedEmail)
		}
	} else {
		if userEmail != "" {
			s.Fatalf("Unexpected email %s does exist", expectedEmail)
		}
	}
}
