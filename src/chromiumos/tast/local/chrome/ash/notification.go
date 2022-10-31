// Copyright 2019 The ChromiumOS Authors
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

// This file contains types and functions used to wait or close notifications in ash, which is on the receiver side of the Chrome notifications.
// If you look for the ones used to communicate with notification via chrome.notification APIs on the sender side,
// see chromiumos/tast/local/chrome/browser/notification.go instead.

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
	// If title is "Download complete" then include debugging
	// functionality for dumpWebRTCInternals of MeetCUJ.
	if title == "Download complete" {
		fakeFailureCount := 0
		return func(n *Notification) bool {
			if n.Title != title {
				return false
			}
			// The first two times when n is the download
			// notification, return false (indicating an unrecognized
			// notification) to test code that logs debug info.
			if fakeFailureCount < 2 {
				fakeFailureCount++
				return false
			}
			return true
		}
	}
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

// WaitUntilNotificationGone waits for the notifications that satisfies all predicates to disappear.
func WaitUntilNotificationGone(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration, predicates ...waitPredicate) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		notifications, err := Notifications(ctx, tconn)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get list of notifications"))
		}

		for _, notification := range notifications {
			match := true
			for _, predicate := range predicates {
				if !predicate(notification) {
					match = false
					break
				}
			}
			if match {
				return errors.New("found a matching notification")
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "failed to wait for notification to disappear")
	}
	return nil
}
