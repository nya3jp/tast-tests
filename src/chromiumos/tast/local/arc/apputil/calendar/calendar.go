// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package calendar contains local Tast tests that exercise calendar.
package calendar

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/apputil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	appName  = "Calendar"
	pkgName  = "com.google.android.calendar"
	idPrefix = pkgName + ":id/"

	longTimeout    = 30 * time.Second
	defaultTimeout = 15 * time.Second
	shortTimeout   = 5 * time.Second
)

// Calendar opens default media app "calendar" from files app.
type calendar struct {
	*apputil.App
}

// NewApp returns calendar instance.
func NewApp(ctx context.Context, kb *input.KeyboardEventWriter, tconn *chrome.TestConn, a *arc.ARC) (*calendar, error) {
	app, err := apputil.NewApp(ctx, kb, tconn, a, appName, pkgName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create app instance")
	}

	return &calendar{app}, err
}

// SkipStart skips start prompt.
func (c *calendar) SkipPrompt(ctx context.Context) error {
	nextPageBtn := c.Device.Object(ui.Description("next page"), ui.ID(idPrefix+"next_arrow"))
	gotItBtn := c.Device.Object(ui.Text("Got it"), ui.ID(idPrefix+"in_page_done_button"))
	allowBtn := c.Device.Object(ui.Text("ALLOW"))

	return uiauto.Combine("skip start prompt",
		apputil.ClickIfExist(nextPageBtn, shortTimeout), // Skip the greeting of Google Calendar.
		apputil.ClickIfExist(nextPageBtn, shortTimeout), // Skip the introduction of Google Calendar.
		apputil.ClickIfExist(gotItBtn, shortTimeout),    // Skip the alert of don't miss a thing.
		apputil.ClickIfExist(allowBtn, shortTimeout),    // Allow Google Calendar to access the user's calendar.
		apputil.ClickIfExist(allowBtn, shortTimeout),    // Allow Google Calendar to access the user's contacts.
	)(ctx)
}

// SelectMonth select display mode to Month.
func (c *calendar) SelectMonth(ctx context.Context) error {
	testing.ContextLog(ctx, "Select display mode to Month")

	listBtn := c.Device.Object(ui.Description("Show Calendar List and Settings drawer"))
	monthBtn := c.Device.Object(ui.DescriptionContains("Month view"))
	// Set three seconds intervals to avoid Calendar app crashes.
	pollOpt := testing.PollOptions{Timeout: longTimeout, Interval: 3 * time.Second}

	// List may not appear if click the listBtn before it is stabilized.
	return uiauto.Combine("open month",
		apputil.RetryUntil(apputil.FindAndClick(listBtn, defaultTimeout), apputil.WaitForExists(monthBtn, 200*time.Microsecond), &pollOpt),
		apputil.RetryUntil(apputil.FindAndClick(monthBtn, defaultTimeout), apputil.WaitUntilGone(monthBtn, 200*time.Microsecond), &pollOpt),
	)(ctx)
}

// JumpToToday clicks jump to today button.
func (c *calendar) JumpToToday(ctx context.Context) error {
	testing.ContextLog(ctx, "Jump to today")
	return apputil.FindAndClick(c.Device.Object(ui.Description("Jump to Today")), defaultTimeout)(ctx)
}

// CreateEvent creates an event of specified name that occurs after an hour and causes the notification to pop up.
func (c *calendar) CreateEvent(ctx context.Context, eventName string) error {
	testing.ContextLogf(ctx, "Create an event of name %s", eventName)

	createBtn := c.Device.Object(ui.Description("Create new event and more"))
	newEvent := c.Device.Object(ui.Description("Event button"))
	saveBtn := c.Device.Object(ui.ID(idPrefix + "save"))

	if err := uiauto.Combine("open event",
		apputil.FindAndClick(createBtn, defaultTimeout),
		apputil.FindAndClick(newEvent, defaultTimeout),
		apputil.WaitForExists(saveBtn, shortTimeout),
	)(ctx); err != nil {
		return err
	}

	if err := c.KB.Type(ctx, eventName); err != nil {
		return errors.Wrapf(err, "failed to type %q", eventName)
	}

	removeBtn := c.Device.Object(ui.Description("Remove notification"))
	addBtn := c.Device.Object(ui.Text("Add notification"))
	notificationTime := c.Device.Object(ui.Text("1 hour before"))
	verifyBtn := c.Device.Object(ui.Text("1 day before"))

	if err := uiauto.Combine("set notification time",
		apputil.ClickIfExist(removeBtn, defaultTimeout),
		apputil.FindAndClick(addBtn, defaultTimeout),
		apputil.FindAndClick(notificationTime, defaultTimeout),
	)(ctx); err != nil {
		return err
	}

	if err := verifyBtn.WaitUntilGone(ctx, shortTimeout); err != nil {
		return errors.Wrap(err, "failed to wait notification time selections gone")
	}

	if err := apputil.FindAndClick(saveBtn, defaultTimeout)(ctx); err != nil {
		return errors.Wrap(err, "failed to save event")
	}

	return saveBtn.WaitUntilGone(ctx, shortTimeout)
}

// DeleteEvent deletes an event with specified name.
func (c *calendar) DeleteEvent(ctx context.Context, eventName string) error {
	testing.ContextLogf(ctx, "Delete an event of name %s", eventName)

	listBtn := c.Device.Object(ui.Description("Show Calendar List and Settings drawer"))
	scheduleBtn := c.Device.Object(ui.ID(idPrefix+"label"), ui.Description("Schedule view"))

	if err := uiauto.Combine("open schedule",
		apputil.FindAndClick(listBtn, defaultTimeout),
		apputil.FindAndClick(scheduleBtn, defaultTimeout),
	)(ctx); err != nil {
		return err
	}

	notificationEvent := c.Device.Object(ui.DescriptionContains(eventName))
	verifyEventOpen := c.Device.Object(ui.ID(idPrefix+"first_line_text"), ui.Text("Events"))
	moreOptions := c.Device.Object(ui.Description("More options"))
	deleteBtn := c.Device.Object(ui.ID(idPrefix+"title"), ui.Text("Delete"))
	deleteOption := c.Device.Object(ui.ID("android:id/button1"), ui.Text("Delete"))

	return uiauto.Combine("delete notification event in schedule",
		apputil.FindAndClick(notificationEvent, defaultTimeout),
		apputil.WaitForExists(verifyEventOpen, shortTimeout),
		apputil.FindAndClick(moreOptions, defaultTimeout),
		apputil.FindAndClick(deleteBtn, defaultTimeout),
		apputil.FindAndClick(deleteOption, defaultTimeout),
	)(ctx)
}
