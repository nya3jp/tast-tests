// Copyright 2022 The ChromiumOS Authors.
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

var (
	// IssueText export is used to enter the issue description.
	IssueText = "I am not able to connect to Bluetooth"
)

// Launch starts the Feedback app via the default method.
func Launch(ctx context.Context, tconn *chrome.TestConn) (*nodewith.Finder, error) {
	if err := apps.Launch(ctx, tconn, apps.Feedback.ID); err != nil {
		return nil, errors.Wrap(err, "failed to launch feedback app")
	}

	if err := ash.WaitForApp(ctx, tconn, apps.Feedback.ID, time.Minute); err != nil {
		return nil, errors.Wrap(err, "feedback app did not appear in shelf after launch")
	}

	ui := uiauto.New(tconn)

	feedbackRootNode := nodewith.Name(apps.Feedback.Name).Role(role.Window)
	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(feedbackRootNode)(
		ctx); err != nil {
		return nil, errors.Wrap(err, "failed to find feedback app")
	}

	return feedbackRootNode, nil
}

// LaunchAndGoToShareDataPage starts the Feedback app via default method and navigates to
// share data page.
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
	if err := uiauto.Combine("Focus text field",
		ui.WaitUntilExists(issueDescriptionInput),
		ui.EnsureFocused(issueDescriptionInput),
	)(ctx); err != nil {
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
	if err := uiauto.Combine("Click continue button",
		ui.WaitUntilExists(button),
		ui.LeftClick(button),
	)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to click continue button")
	}

	return feedbackRootNode, nil
}
