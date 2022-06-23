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

// Launch starts the Feedback app via the default method
func Launch(ctx context.Context, tconn *chrome.TestConn) (*nodewith.Finder, error) {
	if err := apps.Launch(ctx, tconn, apps.Feedback.ID); err != nil {
		return nil, errors.Wrap(err, "failed to launch feedback app")
	}

	if err := ash.WaitForApp(ctx, tconn, apps.Feedback.ID, time.Minute); err != nil {
		return nil, errors.Wrap(err, "feedback app did not appear in shelf after launch")
	}

	ui := uiauto.New(tconn)

	feedbackRootNode := nodewith.Name(apps.Feedback.Name).Role(role.Window)
	if err := ui.WithTimeout(20 * time.Second).WaitUntilExists(feedbackRootNode)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to find feedback app")
	}

	return feedbackRootNode, nil
}
