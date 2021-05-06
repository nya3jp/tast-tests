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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
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
	if err := tconn.Call(ctx, &ret, "tast.promisify(chrome.autotestPrivate.getVisibleNotifications)"); err != nil {
		return nil, errors.Wrap(err, "failed to call getVisibleNotifications")
	}
	return ret, nil
}

// HideVisibleNotifications clicks on the tray button to show and hide the system tray button, which should also hide any visible notification.
func HideVisibleNotifications(ctx context.Context, tconn *chrome.TestConn) error {
	ns, err := Notifications(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get notifications")
	}
	if len(ns) == 0 {
		// No notification to hide.
		return nil
	}
	ui := uiauto.New(tconn)
	trayButton := nodewith.ClassName("UnifiedSystemTray").Role(role.Button)
	settingBubble := nodewith.ClassName("SettingBubbleContainer")
	return uiauto.Combine("hide visible notifications",
		ui.LeftClick(trayButton),
		ui.WithTimeout(2*time.Second).WaitUntilExists(settingBubble),
		ui.LeftClick(trayButton),
	)(ctx)
}

// CloseNotifications clicks on the tray button to show the system tray button,
// clicks close button on each notification and clicks on the tray button
// to hide the system tray button.
func CloseNotifications(ctx context.Context, tconn *chrome.TestConn) error {
	ns, err := Notifications(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get notifications")
	}
	if len(ns) == 0 {
		// No notification to close.
		return nil
	}
	ui := uiauto.New(tconn)
	trayButton := nodewith.ClassName("UnifiedSystemTray").Role(role.Button)
	settingBubble := nodewith.ClassName("SettingBubbleContainer")
	if err := uiauto.Combine("open setting bubble",
		ui.LeftClick(trayButton),
		ui.WithTimeout(2*time.Second).WaitUntilExists(settingBubble),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to open setting bubble")
	}

	clearAll := nodewith.ClassName("LabelButtonLabel").Name("Clear all").Ancestor(nodewith.ClassName("StackedNotificationBar"))
	notificationMsg := nodewith.ClassName("UnifiedMessageListView").Ancestor(nodewith.ClassName("UnifiedMessageListView").First())
	closeBtn := nodewith.Name("Notification close").ClassName("PaddedButton").Ancestor(nodewith.ClassName("NotificationControlButtonsView"))
	for {
		ns, err := Notifications(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get notifications")
		}
		if len(ns) == 0 {
			break
		}
		// Use "Clear all" button if it exists.
		if err := ui.Exists(clearAll)(ctx); err == nil {
			if err := ui.LeftClick(clearAll)(ctx); err == nil {
				break
			}
		}
		// Here we intentionally ignore errors, because the notification UI tree keeps changing.
		// However it's not a problem since we repeat operation until there
		// will be no notifications.
		uiauto.Combine("hover and close notification",
			ui.WithTimeout(2*time.Second).MouseMoveTo(notificationMsg.First(), 0),
			ui.WithTimeout(2*time.Second).LeftClick(closeBtn),
		)(ctx)
	}

	if err := ui.LeftClick(trayButton)(ctx); err != nil {
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

// WaitTitleContains creates a predicate that checks whether the notification's title contains the given text.
func WaitTitleContains(titleContains string) waitPredicate {
	return func(n *Notification) bool {
		return strings.Contains(n.Title, titleContains)
	}
}

// WaitMessageContains creates a predicate that checks whether the notification's message contains the given text.
func WaitMessageContains(messageContains string) waitPredicate {
	return func(n *Notification) bool {
		return strings.Contains(n.Message, messageContains)
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

// NotificationType describes the types of notifications you can create with chrome.notifications.create()
type NotificationType string

// As defined in https://developer.chrome.com/apps/notifications#type-TemplateType
const (
	NotificationTypeBasic    NotificationType = "basic"
	NotificationTypeImage    NotificationType = "image"
	NotificationTypeList     NotificationType = "list"
	NotificationTypeProgress NotificationType = "progress"
)

// CreateTestNotification creates a notification with a custom title and message.
// iconUrl is a required field to the chrome.notifiations.create() call so a 1px transparent data-url is hardcoded.
func CreateTestNotification(ctx context.Context, tconn *chrome.TestConn, notificationType NotificationType, title, message string) (string, error) {
	var id string
	if err := tconn.Call(ctx, &id,
		`async (notificationType, title, message, iconUrl) =>
		tast.promisify(chrome.notifications.create)({
			type: notificationType,
			title: title,
			message: message,
			iconUrl: iconUrl
		})`,
		notificationType,
		title,
		message,
		"data:image/gif;base64,R0lGODlhAQABAIAAAP///wAAACH5BAEAAAAALAAAAAABAAEAAAICRAEAOw=="); err != nil {
		return "", errors.Wrap(err, "failed to create notification")
	}
	return id, nil
}
