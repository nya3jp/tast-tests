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
	"chromiumos/tast/local/apps"
	fpb "chromiumos/tast/local/bundles/cros/feedback/proto"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/feedbackapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const cameraCategoryTag = "chromeos-camera-app"

func init() {
	testing.AddTest(&testing.Test{
		Func:         VerifyCategoryTag,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify category_tag value in the report",
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
	})
}

// VerifyCategoryTag verifies the category_tag value in the report. Open Feedback
// app from the Camera app, the category_tag in the report should be chromeos-camera-app.
func VerifyCategoryTag(ctx context.Context, s *testing.State) {
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

	// Open Camera app.
	if err := apps.Launch(ctx, tconn, apps.Camera.ID); err != nil {
		s.Fatal("Failed to launch the Camera app: ", err)
	}
	if err := ash.WaitForApp(ctx, tconn, apps.Camera.ID, time.Minute); err != nil {
		s.Fatal("Failed to wait for the Camera app")
	}

	// Open feedback app in the Camera app.
	settingsButton := nodewith.Name("Settings")
	sendFeedbackButton := nodewith.Name("Send feedback").Role(role.Button)
	if err := ui.DoDefault(settingsButton)(ctx); err != nil {
		s.Fatal("Failed to click Settings button: ", err)
	}
	if err := uiauto.Combine("Open feedback app in the Camera app",
		ui.DoDefault(settingsButton),
		ui.WaitUntilExists(sendFeedbackButton),
		ui.DoDefault(sendFeedbackButton),
	)(ctx); err != nil {
		s.Fatal("Failed to open feedback app in the Camera app: ", err)
	}

	// Verify Feedback app is launched.
	if err = ash.WaitForApp(ctx, tconn, apps.Feedback.ID, time.Minute); err != nil {
		s.Fatal("Could not find app in shelf after launch: ", err)
	}

	// Find the issue description text input.
	issueDescriptionInput := nodewith.Role(role.TextField)
	if err := ui.EnsureFocused(issueDescriptionInput)(ctx); err != nil {
		s.Fatal("Failed to find the issue description text input: ", err)
	}

	// Set up keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	// Type issue description.
	if err := kb.Type(ctx, feedbackapp.IssueText); err != nil {
		s.Fatal("Failed to type issue description: ", err)
	}

	// Find continue button and click.
	button := nodewith.Name("Continue").Role(role.Button)
	if err := ui.DoDefault(button)(ctx); err != nil {
		s.Fatal("Failed to click continue button")
	}

	// Submit the feedback and verify confirmation page title exists.
	sendButton := nodewith.Name("Send").Role(role.Button)
	confirmationPageTitle := nodewith.Name("Thanks for your feedback").Role(
		role.StaticText)

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

	// Verify the category_tag value in the report.
	report := &fpb.ExtensionSubmit{}
	if err = proto.Unmarshal(content, report); err != nil {
		s.Fatal("Failed to parse report: ", err)
	}
	categoryTag := report.GetBucket()
	if categoryTag != cameraCategoryTag {
		s.Fatal("Failed to get the correct camera category tag")
	}
}
