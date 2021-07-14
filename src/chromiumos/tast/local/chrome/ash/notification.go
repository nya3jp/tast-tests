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

// CloseNotifications uses autotestPrivate api to close all notifications.
func CloseNotifications(ctx context.Context, tconn *chrome.TestConn) error {
	return tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.removeAllNotifications)")
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

// NotificationItem describes an individual item in a list notification.
// As defined in https://developer.chrome.com/docs/extensions/reference/notifications/#type-NotificationItem
type NotificationItem struct {
	Message string `json:"message"`
	Title   string `json:"title"`
}

// CreateTestNotification creates a notification with a custom title and message.
// iconUrl is a required field to the chrome.notifiations.create() call so a 1px transparent data-url is hardcoded.
func CreateTestNotification(ctx context.Context, tconn *chrome.TestConn, notificationType NotificationType, title, message string) (string, error) {
	var id string
	var imageURL string
	var items []NotificationItem

	if notificationType == NotificationTypeImage {
		// Used a transparent solid color image for testing.
		imageURL = "data:image/gif;base64,iVBORw0KGgoAAAANSUhEUgAAAAoAAAAKCAYAAACNMs+9AAAAFUlEQVR42mNk+P+/noEIwDiqkL4KAbERGO3PogdhAAAAAElFTkSuQmCC"
	}

	if notificationType == NotificationTypeList {
		// Used 2 mock items for testing.
		items = []NotificationItem{{Message: "item1", Title: "title1"}, {Message: "item2", Title: "title2"}}
	}

	if err := tconn.Call(ctx, &id,
		`async (notificationType, title, message, iconUrl, imageURL, items) =>
		tast.promisify(chrome.notifications.create)({
			type: notificationType,
			title: title,
			message: message,
			iconUrl: iconUrl,
			imageUrl: imageURL,
			items: items
		})`,
		notificationType,
		title,
		message,
		"data:image/gif;base64,R0lGODlhAQABAIAAAP///wAAACH5BAEAAAAALAAAAAABAAEAAAICRAEAOw==",
		imageURL,
		items); err != nil {
		return "", errors.Wrap(err, "failed to create notification")
	}
	return id, nil
}

// ClearNotification clear a notification with the given id.
func ClearNotification(ctx context.Context, tconn *chrome.TestConn, id string) error {
	if err := tconn.Call(ctx, nil,
		`async (id) =>
		tast.promisify(chrome.notifications.clear)(id)`,
		id); err != nil {
		return errors.Wrap(err, "failed to clear notification")
	}
	return nil
}
