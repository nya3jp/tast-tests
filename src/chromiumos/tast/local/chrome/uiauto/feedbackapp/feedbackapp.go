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
)

var (
	// feedbackRootNodeParams export is used to find the root node of Feedback app.
	feedbackRootNodeParams = nodewith.Name(apps.Feedback.Name).Role(role.Window)
)

// FeedbackRootNode returns the root ui node of Feedback app.
func FeedbackRootNode(ctx context.Context, tconn *chrome.TestConn) (*nodewith.Finder, error) {
	ui := uiauto.New(tconn)
	err := ui.WithTimeout(20 * time.Second).WaitUntilExists(feedbackRootNodeParams)(ctx)
	return feedbackRootNodeParams, err
}

// Launch Feedback app via default method.
func Launch(ctx context.Context, tconn *chrome.TestConn) (*nodewith.Finder, error) {
	err := apps.Launch(ctx, tconn, apps.Feedback.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch feedback app")
	}

	err = ash.WaitForApp(ctx, tconn, apps.Feedback.ID, time.Minute)
	if err != nil {
		return nil, errors.Wrap(err, "feedback app did not appear in shelf after launch")
	}

	feedbackRootNode, err := FeedbackRootNode(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find feedback app")
	}
	return feedbackRootNode, nil
}
