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
		Func:         VerifyEssentialData,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify description and screenshot values in the report",
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
			Name: "include_screenshot",
			Val:  true,
		}, {
			Name: "not_include_screenshot",
			Val:  false,
		}},
	})
}

// VerifyEssentialData verifies if description and screenshot are
// in the report in different conditions. Description should always be
// included in the report and screenshot should be included in the report
// when the screenshot checkbox is checked.
func VerifyEssentialData(ctx context.Context, s *testing.State) {
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

	// Clean up in both beginning and the end.
	cleanUp := func() {
		if err := os.RemoveAll(feedbackapp.ReportPath); err != nil {
			s.Error("Failed to remove feedback report: ", err)
		}
	}
	cleanUp()
	defer cleanUp()

	ui := uiauto.New(tconn).WithTimeout(20 * time.Second)
	includeScreenshot := s.Param().(bool)

	// Launch feedback app and go to share data page.
	feedbackRootNode, err := feedbackapp.LaunchAndGoToShareDataPage(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch feedback app and navigate to share data page: ", err)
	}

	// Check the screenshot checkbox if needed.
	if includeScreenshot {
		checkbox := nodewith.Role(role.CheckBox).Ancestor(feedbackRootNode).First()
		if err := uiauto.Combine("Verify checkbox is unchecked and click it",
			ui.WaitUntilExists(checkbox.Attribute("checked", "false")),
			ui.DoDefault(checkbox),
			ui.WaitUntilExists(checkbox.Attribute("checked", "true")),
		)(ctx); err != nil {
			s.Fatal("Failed to change the checkbox state: ", err)
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

	// Get description and screenshot to verify their values.
	reportDescription := report.GetCommonData().GetDescription()
	reportScreenshot := report.GetScreenshot()

	if reportDescription != feedbackapp.IssueText {
		s.Fatal("Failed to get correct report description")
	}
	if includeScreenshot {
		if reportScreenshot == nil {
			s.Fatal("Failed to include the screenshot in the report")
		}
	} else {
		if reportScreenshot != nil {
			s.Fatal("Failed to not include the screenshot in the report")
		}
	}
}
