// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"
)

// Object is a representation of an Android view.
//
// An instantiated Object does NOT uniquely identify an Android view. Instead,
// it holds a selector to locate a matching view when its methods are called.
// Once you create an instance of Object, it can be reused for different views
// matching the selector.
//
// This object corresponds to UiObject in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiObject
type Object struct {
	d *Device
	s *selector
}

type objectInfo struct {
	Text string `json:"text"`
}

// Object creates an Object from given selectors.
//
// Example:
//  btn := d.Object(ui.ID("foo_button"), ui.Text("bar"))
func (d *Device) Object(opts ...SelectorOption) *Object {
	return &Object{d: d, s: newSelector(opts)}
}

// WaitForExists waits for a view matching the selector to appear.
//
// This method corresponds to UiObject.waitForExists().
// https://developer.android.com/reference/android/support/test/uiautomator/UiObject.html#waitForExists(long)
func (o *Object) WaitForExists(ctx context.Context) error {
	var success bool
	if err := o.d.call(ctx, "waitForExists", &success, o.s, getTimeoutMs(ctx)); err != nil {
		return err
	}
	if !success {
		return fmt.Errorf("waitForExists failed: %v", o.s)
	}
	return nil
}

// Click clicks a view matching the selector.
//
// This method corresponds to UiObject.click().
// https://developer.android.com/reference/android/support/test/uiautomator/UiObject.html#click
func (o *Object) Click(ctx context.Context) error {
	var success bool
	if err := o.d.call(ctx, "click", &success, o.s); err != nil {
		return err
	}
	if !success {
		return fmt.Errorf("click failed: %v", o.s)
	}
	return nil
}

// GetText gets the text property of a view.
//
// This method corresponds to UiObject.getText().
// https://developer.android.com/reference/android/support/test/uiautomator/UiObject.html#gettext
func (o *Object) GetText(ctx context.Context) (string, error) {
	info, err := o.info(ctx)
	if err != nil {
		return "", err
	}
	return info.Text, nil
}

// SetText sets the text property of a view.
//
// This method corresponds to UiObject.setText().
// https://developer.android.com/reference/android/support/test/uiautomator/UiObject.html#settext
func (o *Object) SetText(ctx context.Context, s string) error {
	var success bool
	if err := o.d.call(ctx, "setText", &success, o.s, s); err != nil {
		return err
	}
	if !success {
		return fmt.Errorf("setText failed: %v", o.s)
	}
	return nil
}

func (o *Object) info(ctx context.Context) (*objectInfo, error) {
	var info objectInfo
	if err := o.d.call(ctx, "objInfo", &info, o.s); err != nil {
		return nil, err
	}
	return &info, nil
}

// getTimeoutMs returns the remaining timeout of ctx in milliseconds.
func getTimeoutMs(ctx context.Context) int64 {
	d, ok := ctx.Deadline()
	if !ok {
		return 1000000000000 // long enough timeout
	}
	t := d.Sub(time.Now()).Seconds()
	if t < 0 {
		return 1
	}
	return int64(t * 1000)
}
