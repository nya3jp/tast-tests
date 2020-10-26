// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"strings"
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

// Notifications returns an array of notifications in Chrome.
// tconn must be the connection returned by chrome.TestAPIConn().
//
// Note: it uses an autotestPrivate API with the misleading name
// getVisibleNotifications under the hood.
func Notifications(ctx context.Context, tconn *chrome.TestConn) ([]*Notification, error) {
	var ret []*Notification
	if err := tconn.EvalPromise(ctx,
		`tast.promisify(chrome.autotestPrivate.getVisibleNotifications)()`, &ret); err != nil {
		return nil, errors.Wrap(err, "failed to call getVisibleNotifications")
	}
	return ret, nil
}

// HideVisibleNotifications clicks on the tray button to show and hide the system tray button, which should also hide any visible notification.
func HideVisibleNotifications(ctx context.Context, tconn *chrome.TestConn) error {
	trayButton, err := ui.Find(ctx, tconn, ui.FindParams{Role: ui.RoleTypeButton, ClassName: "UnifiedSystemTray"})
	if err != nil {
		return errors.Wrap(err, "system tray button not found")
	}
	defer trayButton.Release(ctx)

	if err := mouse.Click(ctx, tconn, trayButton.Location.CenterPoint(), mouse.LeftButton); err != nil {
		return errors.Wrap(err, "failed to click the tray button")
	}

	if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{ClassName: "SettingBubbleContainer"}, 2*time.Second); err != nil {
		return errors.Wrap(err, "quick settings does not appear")
	}

	if err := mouse.Click(ctx, tconn, trayButton.Location.CenterPoint(), mouse.LeftButton); err != nil {
		return errors.Wrap(err, "failed to click the tray button")
	}
	return nil
}

// CloseNotifications clicks on the tray button to show the system tray button,
// clicks close button on each notification and clicks on the tray button
// to hide the system tray button.
func CloseNotifications(ctx context.Context, tconn *chrome.TestConn) error {
	trayButton, err := ui.Find(ctx, tconn, ui.FindParams{Role: ui.RoleTypeButton, ClassName: "UnifiedSystemTray"})
	if err != nil {
		return errors.Wrap(err, "system tray button not found")
	}
	defer trayButton.Release(ctx)

	if err := mouse.Click(ctx, tconn, trayButton.Location.CenterPoint(), mouse.LeftButton); err != nil {
		return errors.Wrap(err, "failed to click the tray button")
	}

	if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{ClassName: "SettingBubbleContainer"}, 2*time.Second); err != nil {
		return errors.Wrap(err, "quick settings does not appear")
	}

	params := ui.FindParams{
		Name:      "Notification close",
		ClassName: "ImageButton",
		Role:      ui.RoleTypeButton,
	}

	for {
		nodes, err := ui.FindAll(ctx, tconn, params)
		if err != nil {
			return errors.Wrap(err, "failed get list of all notifications close buttons")
		}
		if len(nodes) == 0 {
			break
		}

		for _, node := range nodes {
			// Here we intentionaly ignore errors, because we modify
			// accesability tree by clicking close buttons.
			// However it's not a problem since we repeat operation until there
			// will be no close buttons.
			node.LeftClick(ctx)
		}
		nodes.Release(ctx)
	}

	if err := mouse.Click(ctx, tconn, trayButton.Location.CenterPoint(), mouse.LeftButton); err != nil {
		return errors.Wrap(err, "failed to click the tray button")
	}

	return nil
}

// waitPredicate is a function that returns true if notification satisfies some
// conditions.
type waitPredicate func(n *Notification) bool

// WaitIDContains creates a predicate that checks whether notification ID
// contains idContains.
func WaitIDContains(idContains string) waitPredicate {
	return func(n *Notification) bool {
		return strings.Contains(n.ID, idContains)
	}
}

// WaitTitle creates a predicate that checks whether notification has specific
// title.
func WaitTitle(title string) waitPredicate {
	return func(n *Notification) bool {
		return n.Title == title
	}
}

// WaitForNotification waits for the first notification that satisfies all wait
// predicates.
func WaitForNotification(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration, predicates ...waitPredicate) (*Notification, error) {
	var result *Notification

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		notifications, err := Notifications(ctx, tconn)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get list of notifications"))
		}

	NotificationLoop:
		for _, notification := range notifications {
			for _, predicate := range predicates {
				if !predicate(notification) {
					continue NotificationLoop
				}
			}
			result = notification
			return nil
		}
		return errors.New("no wanted notification")
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return nil, errors.Wrap(err, "failed to wait for notification")
	}
	return result, nil
}
