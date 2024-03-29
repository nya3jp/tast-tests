// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package feedbackapp contains drivers for controlling the ui of feedback SWA.
package feedbackapp

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
)

const numberOfHelpLinks = 5

// IssueText export is used to enter the issue description.
const IssueText = "Test only - please ignore"

// ReportPath export is used to find the feedback report path.
const ReportPath = "/tmp/feedback-report/feedback-report"

// PngFile and PdfFile are used to define the file names.
const (
	PngFile = "attach_file_upload_01.png"
	PdfFile = "attach_file_upload_02.pdf"
)

// Launch starts the Feedback app via the default method.
func Launch(ctx context.Context, tconn *chrome.TestConn) (*nodewith.Finder, error) {
	if err := apps.Launch(ctx, tconn, apps.Feedback.ID); err != nil {
		return nil, errors.Wrap(err, "failed to launch feedback app")
	}

	if err := ash.WaitForApp(ctx, tconn, apps.Feedback.ID, time.Minute); err != nil {
		return nil, errors.Wrap(err, "feedback app did not appear in shelf after launch")
	}

	ui := uiauto.New(tconn).WithTimeout(20 * time.Second)

	feedbackRootNode := nodewith.Name(apps.Feedback.Name).Role(role.Window)
	if err := ui.WaitUntilExists(feedbackRootNode)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to find feedback app")
	}

	return feedbackRootNode, nil
}

// LaunchAndGoToShareDataPage starts the Feedback app via default method and navigates to
// share data page. This function returns the feedback app root node and possible error.
func LaunchAndGoToShareDataPage(ctx context.Context, tconn *chrome.TestConn) (
	*nodewith.Finder, error) {
	// Launch Feedback app.
	feedbackRootNode, err := Launch(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch feedback app")
	}

	ui := uiauto.New(tconn)

	// Find issue description input.
	issueDescriptionInput := nodewith.Role(role.TextField).Ancestor(feedbackRootNode)
	if err := ui.EnsureFocused(issueDescriptionInput)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to find the issue description text input")
	}

	// Set up keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find keyboard")
	}
	defer kb.Close()

	// Enter issue description.
	if err := kb.Type(ctx, IssueText); err != nil {
		return nil, errors.Wrap(err, "failed to enter issue description")
	}

	// Find continue button and click.
	button := nodewith.Name("Continue").Role(role.Button).Ancestor(feedbackRootNode)
	if err := ui.DoDefault(button)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to click continue button")
	}

	return feedbackRootNode, nil
}

// LaunchAndGoToConfirmationPage starts Feedback app via default method and navigate to
// confirmation page. This function returns the feedback app root node and possible error.
func LaunchAndGoToConfirmationPage(ctx context.Context, tconn *chrome.TestConn) (
	*nodewith.Finder, error) {
	// Launch Feedback app and navigate to share data page
	feedbackRootNode, err := LaunchAndGoToShareDataPage(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch feedback app")
	}

	ui := uiauto.New(tconn)

	// Find send button and submit the feedback.
	sendButton := nodewith.Name("Send").Role(role.Button).Ancestor(feedbackRootNode)
	if err := ui.DoDefault(sendButton)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to click send button")
	}

	return feedbackRootNode, nil
}

// VerifyFeedbackAppIsLaunched checks that the app is open and all essential elements are in place.
func VerifyFeedbackAppIsLaunched(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context) error {
	if err := ash.WaitForApp(ctx, tconn, apps.Feedback.ID, time.Minute); err != nil {
		return errors.Wrap(err, "could not find app in shelf after launch")
	}

	// Verify essential elements exist.
	issueDescriptionInput := nodewith.NameContaining("Description").Role(role.TextField)
	button := nodewith.Name("Continue").Role(role.Button)

	if err := uiauto.Combine("Verify essential elements exist",
		ui.WaitUntilExists(issueDescriptionInput),
		ui.WaitUntilExists(button),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to find element")
	}

	// Verify five default help content links exist.
	helpLink := nodewith.Role(role.Link).Ancestor(nodewith.Role(role.Iframe))
	for i := 0; i < numberOfHelpLinks; i++ {
		item := helpLink.Nth(i)
		if err := ui.WaitUntilExists(item)(ctx); err != nil {
			return errors.Wrap(err, "failed to find five help links")
		}
	}

	return nil
}
