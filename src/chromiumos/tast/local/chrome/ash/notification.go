// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/testing"
)

// Notification corresponds to the "Notification" defined in
// autotest_private.idl.
type Notification struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Message  string `json:"message"`
	Priority int    `json:"priority"`
	Progress int    `json:"progress"`
}

// VisibleNotifications returns an array of visible notifications in Chrome.
// tconn must be the connection returned by chrome.TestAPIConn().
func VisibleNotifications(ctx context.Context, tconn *chrome.TestConn) ([]*Notification, error) {
	var ret []*Notification
	if err := tconn.EvalPromise(ctx,
		`tast.promisify(chrome.autotestPrivate.getVisibleNotifications)()`, &ret); err != nil {
		return nil, errors.Wrap(err, "failed to call getVisibleNotifications")
	}
	return ret, nil
}

// DismissFloatingNotifications dismisses floating notifications if any.
func DismissFloatingNotifications(ctx context.Context, tconn *chrome.TestConn) error {
	root, err := ui.Root(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get root node")
	}
	defer root.Release(ctx)

	params := ui.FindParams{ClassName: "MessageView"}
	for {
		exists, err := root.DescendantExists(ctx, params)
		if err != nil {
			return errors.Wrap(err, "failed to check existence of MessageView")
		}
		if !exists {
			break
		}

		m, err := root.Descendant(ctx, params)
		if err != nil {
			return errors.Wrap(err, "failed to get MessageView")
		}
		defer m.Release(ctx)

		// Move mouse to hover the notification.
		if err := mouse.Move(ctx, tconn, m.Location.CenterPoint(), 200*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to move mouse")
		}

		// Wait for the close button to show up.
		c, err := m.DescendantWithTimeout(ctx, ui.FindParams{Name: "Notification close"}, time.Second)
		if err != nil {
			return errors.Wrap(err, "failed to find close button")
		}

		// Click the close button.
		if err := c.LeftClick(ctx); err != nil {
			return errors.Wrap(err, "failed to click close button")
		}

		// Wait for the notification to go away before checking for the next.
		if err := testing.Sleep(ctx, time.Second); err != nil {
			return errors.Wrap(err, "failed to sleep")
		}
	}

	return nil
}
