// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package resizelockhelper provides resize lock Helper functions.
package resizelockhelper

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	chromeui "chromiumos/tast/local/chrome/ui"
)

const (
	// Used to (i) find the resize lock mode buttons on the compat-mode menu and (ii) check the state of the compat-mode button
	PhoneButtonName     = "Phone"
	TabletButtonName    = "Tablet"
	ResizableButtonName = "Resizable"

	// Currently the automation API doesn't support unique ID, so use the classnames to find the elements of interest.
	CenterButtonClassName  = "FrameCenterButton"
	CheckBoxClassName      = "Checkbox"
	BubbleDialogClassName  = "BubbleDialogDelegateView"
	OverlayDialogClassName = "OverlayDialog"
	ShelfIconClassName     = "ash/ShelfAppButton"
	MenuItemViewClassName  = "MenuItemView"

	// A11y names are available for some UI elements
	SplashCloseButtonName          = "Got it"
	ConfirmButtonName              = "Allow"
	CancelButtonName               = "Cancel"
	AppManagementSettingToggleName = "Preset window sizes"
	AppInfoMenuItemViewName        = "App info"
	CloseMenuItemViewName          = "Close"
)

// CheckVisibility checks whether the node specified by the given class name exists or not.
func CheckVisibility(ctx context.Context, tconn *chrome.TestConn, className string, visible bool) error {
	if visible {
		return chromeui.WaitUntilExists(ctx, tconn, chromeui.FindParams{ClassName: className}, 10*time.Second)
	}
	return chromeui.WaitUntilGone(ctx, tconn, chromeui.FindParams{ClassName: className}, 10*time.Second)
}
