// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/coords"
)

// Available RPC methods are listed at:
// https://github.com/xiaocong/android-uiautomator-server/blob/master/app/src/androidTest/java/com/github/uiautomator/stub/AutomatorService.java

var errTimeout = errors.New("timeout")

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

// rect represents a rectangle, as defined here:
// https://developer.android.com/reference/android/graphics/Rect
// It differs from coords.Rect in that it uses bottom & right instead of width & height.
type rect struct {
	Top    int `json:"top"`
	Left   int `json:"left"`
	Bottom int `json:"bottom"`
	Right  int `json:"right"`
}

// objectInfo represents a collection of attributes that belongs to the object as defined
// by UI Automator server. See:
// https://github.com/lnishan/android-uiautomator-server/blob/master/app/src/androidTest/java/com/github/uiautomator/stub/ObjInfo.java
type objectInfo struct {
	Text               string `json:"text"`
	ContentDescription string `json:"contentDescription"`
	PackageName        string `json:"packageName"`
	ClassName          string `json:"className"`
	Bounds             rect   `json:"bounds"`
	Checkable          bool   `json:"checkable"`
	Checked            bool   `json:"checked"`
	Clickable          bool   `json:"clickable"`
	Enabled            bool   `json:"enabled"`
	Focusable          bool   `json:"focusable"`
	Focused            bool   `json:"focused"`
	LongClickable      bool   `json:"longClickable"`
	Scrollable         bool   `json:"scrollable"`
	Selected           bool   `json:"selected"`
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
func (o *Object) WaitForExists(ctx context.Context, timeout time.Duration) error {
	return o.callSimple(ctx, "waitForExists", o.s, timeout/time.Millisecond)
}

// WaitForText waits for a text view matching the selector to have the expected text.
func (o *Object) WaitForText(ctx context.Context, expected string, timeout time.Duration) error {
	s := *o.s
	s.Text = expected
	s.Mask |= maskText
	terr := o.callSimple(ctx, "waitForExists", &s, timeout/time.Millisecond)
	if terr != nil {
		if actual, err := o.GetText(ctx); err == nil {
			return errors.Errorf("text does not match: got %q; want %q", actual, expected)
		}
		return terr
	}
	return nil
}

// WaitUntilGone waits for a view matching the selector to disappear.
//
// This method corresponds to UiObject.waitUntilGone().
// https://developer.android.com/reference/android/support/test/uiautomator/UiObject.html#waitUntilGone(long)
func (o *Object) WaitUntilGone(ctx context.Context, timeout time.Duration) error {
	return o.callSimple(ctx, "waitUntilGone", o.s, timeout/time.Millisecond)
}

// Click clicks a view matching the selector.
//
// This method corresponds to UiObject.click().
// https://developer.android.com/reference/android/support/test/uiautomator/UiObject.html#click
func (o *Object) Click(ctx context.Context) error {
	return o.callSimple(ctx, "click", o.s)
}

// ScrollTo performs a forward scroll action to move through the scrollable layout element until a view matching the target selector is found.
//
// This method corresponds to UiScrollable.scrollintoview().
// https://developer.android.com/reference/androidx/test/uiautomator/UiScrollable.html#scrollintoview
func (o *Object) ScrollTo(ctx context.Context, target *Object) error {
	return o.callSimple(ctx, "scrollTo", o.s, target.s, true)
}

// Exists returns if an object exists.
//
// This method corresponds to UiObject.exists().
// https://developer.android.com/reference/android/support/test/uiautomator/UiObject.html#exists
func (o *Object) Exists(ctx context.Context) error {
	return o.callSimple(ctx, "exist", o.s)
}

// SetText sets the text property of a view.
//
// This method corresponds to UiObject.setText().
// https://developer.android.com/reference/android/support/test/uiautomator/UiObject.html#settext
func (o *Object) SetText(ctx context.Context, s string) error {
	return o.callSimple(ctx, "setText", o.s, s)
}

// GetText returns the text property of a view.
//
// This method corresponds to UiObject.getText().
// https://developer.android.com/reference/android/support/test/uiautomator/UiObject.html#gettext
func (o *Object) GetText(ctx context.Context) (string, error) {
	info, err := o.info(ctx)
	if err != nil {
		return "", errors.Wrap(err, "GetText failed")
	}
	return info.Text, nil
}

// GetContentDescription returns the content description property of a view.
//
// This method corresponds to UiObject.getContentDescription().
// https://developer.android.com/reference/android/support/test/uiautomator/UiObject.html#getcontentdescription
func (o *Object) GetContentDescription(ctx context.Context) (string, error) {
	info, err := o.info(ctx)
	if err != nil {
		return "", errors.Wrap(err, "GetContentDescription failed")
	}
	return info.ContentDescription, nil
}

// GetPackageName returns the package name of a view.
//
// This method corresponds to UiObject.getPackageName().
// https://developer.android.com/reference/android/support/test/uiautomator/UiObject.html#getpackagename
func (o *Object) GetPackageName(ctx context.Context) (string, error) {
	info, err := o.info(ctx)
	if err != nil {
		return "", errors.Wrap(err, "GetPackageName failed")
	}
	return info.PackageName, nil
}

// GetClassName returns the class name of a view.
//
// This method corresponds to UiObject.getClassName().
// https://developer.android.com/reference/android/support/test/uiautomator/UiObject.html#getclassname
func (o *Object) GetClassName(ctx context.Context) (string, error) {
	info, err := o.info(ctx)
	if err != nil {
		return "", errors.Wrap(err, "GetClassName failed")
	}
	return info.ClassName, nil
}

// GetBounds returns the bounds of a view.
//
// This method corresponds to UiObject.getBounds().
// https://developer.android.com/reference/android/support/test/uiautomator/UiObject.html#getbounds
func (o *Object) GetBounds(ctx context.Context) (coords.Rect, error) {
	info, err := o.info(ctx)
	if err != nil {
		return coords.Rect{}, errors.Wrap(err, "GetBounds failed")
	}
	// Bounds contains a "graphics rect". Needs to be converted to an coords.Rect.
	gr := info.Bounds
	r := coords.NewRectLTRB(gr.Left, gr.Top, gr.Right, gr.Bottom)
	return r, nil
}

// IsCheckable returns if a view is checkable.
//
// This method corresponds to UiObject.isCheckable().
// https://developer.android.com/reference/android/support/test/uiautomator/UiObject.html#ischeckable
func (o *Object) IsCheckable(ctx context.Context) (bool, error) {
	info, err := o.info(ctx)
	if err != nil {
		return false, errors.Wrap(err, "IsCheckable failed")
	}
	return info.Checkable, nil
}

// IsChecked returns if a view is checked.
//
// This method corresponds to UiObject.isChecked().
// https://developer.android.com/reference/android/support/test/uiautomator/UiObject.html#ischecked
func (o *Object) IsChecked(ctx context.Context) (bool, error) {
	info, err := o.info(ctx)
	if err != nil {
		return false, errors.Wrap(err, "IsChecked failed")
	}
	return info.Checked, nil
}

// IsClickable returns if a view is clickable.
//
// This method corresponds to UiObject.isClickable().
// https://developer.android.com/reference/android/support/test/uiautomator/UiObject.html#isclickable
func (o *Object) IsClickable(ctx context.Context) (bool, error) {
	info, err := o.info(ctx)
	if err != nil {
		return false, errors.Wrap(err, "IsClickable failed")
	}
	return info.Clickable, nil
}

// IsEnabled returns if a view is enabled.
//
// This method corresponds to UiObject.isEnabled().
// https://developer.android.com/reference/android/support/test/uiautomator/UiObject.html#isenabled
func (o *Object) IsEnabled(ctx context.Context) (bool, error) {
	info, err := o.info(ctx)
	if err != nil {
		return false, errors.Wrap(err, "IsEnabled failed")
	}
	return info.Enabled, nil
}

// IsFocusable returns if a view is focusable.
//
// This method corresponds to UiObject.isFocusable().
// https://developer.android.com/reference/android/support/test/uiautomator/UiObject.html#isfocusable
func (o *Object) IsFocusable(ctx context.Context) (bool, error) {
	info, err := o.info(ctx)
	if err != nil {
		return false, errors.Wrap(err, "IsFocusable failed")
	}
	return info.Focusable, nil
}

// IsFocused returns if a view is focused.
//
// This method corresponds to UiObject.isFocused().
// https://developer.android.com/reference/android/support/test/uiautomator/UiObject.html#isfocused
func (o *Object) IsFocused(ctx context.Context) (bool, error) {
	info, err := o.info(ctx)
	if err != nil {
		return false, errors.Wrap(err, "IsFocused failed")
	}
	return info.Focused, nil
}

// IsLongClickable returns if a view is longClickable.
//
// This method corresponds to UiObject.isLongClickable().
// https://developer.android.com/reference/android/support/test/uiautomator/UiObject.html#islongclickable
func (o *Object) IsLongClickable(ctx context.Context) (bool, error) {
	info, err := o.info(ctx)
	if err != nil {
		return false, errors.Wrap(err, "IsLongClickable failed")
	}
	return info.LongClickable, nil
}

// IsScrollable returns if a view is scrollable.
//
// This method corresponds to UiObject.isScrollable().
// https://developer.android.com/reference/android/support/test/uiautomator/UiObject.html#isscrollable
func (o *Object) IsScrollable(ctx context.Context) (bool, error) {
	info, err := o.info(ctx)
	if err != nil {
		return false, errors.Wrap(err, "IsScrollable failed")
	}
	return info.Scrollable, nil
}

// IsSelected returns if a view is selected.
//
// This method corresponds to UiObject.isSelected().
// https://developer.android.com/reference/android/support/test/uiautomator/UiObject.html#isselected
func (o *Object) IsSelected(ctx context.Context) (bool, error) {
	info, err := o.info(ctx)
	if err != nil {
		return false, errors.Wrap(err, "IsSelected failed")
	}
	return info.Selected, nil
}

// callSimple is a common method to call a RPC method that returns a boolean indicating success.
func (o *Object) callSimple(ctx context.Context, method string, params ...interface{}) error {
	var success bool
	if err := o.d.call(ctx, method, &success, params...); err != nil {
		return wrapMethodError(method, o.s, err)
	}
	if !success {
		return wrapMethodError(method, o.s, errTimeout)
	}
	return nil
}

// info returns an objectInfo of a view matched by the selector.
func (o *Object) info(ctx context.Context) (*objectInfo, error) {
	const method = "objInfo"
	var info objectInfo
	if err := o.d.call(ctx, method, &info, o.s); err != nil {
		return nil, wrapMethodError(method, o.s, err)
	}
	return &info, nil
}

// wrapMethodError wraps an error returned from an RPC method.
func wrapMethodError(method string, s *selector, err error) error {
	return errors.Wrapf(err, "%s (selector=%v) failed", method, s)
}
