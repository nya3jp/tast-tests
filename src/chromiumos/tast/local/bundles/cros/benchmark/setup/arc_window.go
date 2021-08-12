// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

// DismissMobilePrompt dismisses the prompt of "This app is designed for mobile".
func DismissMobilePrompt(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)

	prompt := nodewith.Name("This app is designed for mobile").Role(role.Window)
	if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(prompt)(ctx); err == nil {
		testing.ContextLog(ctx, "Dismiss the app prompt")
		gotIt := nodewith.Name("Got it").Role(role.Button).Ancestor(prompt)
		if err := ui.LeftClickUntil(gotIt, ui.WithTimeout(time.Second).WaitUntilGone(gotIt))(ctx); err != nil {
			return errors.Wrap(err, "failed to click 'Got it' button")
		}
	}
	return nil
}

// SetResizable sets the ARC APP window to be resizable.
func SetResizable(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)

	centerBtn := nodewith.Role(role.Button).ClassName("FrameCenterButton")
	if err := ui.Exists(centerBtn)(ctx); err != nil {
		return nil
	}
	centerBtnInfo, err := ui.Info(ctx, centerBtn)
	if err != nil {
		return errors.Wrap(err, "failed to get center button info")
	}
	resizable := "Resizable"
	if centerBtnInfo.Name == resizable {
		return nil
	}
	testing.ContextLog(ctx, "Change ARC window to be resizable")
	resizableBtn := nodewith.Role(role.MenuItem).Name(resizable).ClassName("Button")
	if err := ui.LeftClickUntil(centerBtn, ui.WithTimeout(time.Second).WaitUntilExists(resizableBtn))(ctx); err != nil {
		return errors.Wrap(err, "failed to click the center button to show menu items")
	}
	if err := ui.LeftClick(resizableBtn)(ctx); err != nil {
		return errors.Wrap(err, "failed to click the Resizable option")
	}
	allowWin := nodewith.Role(role.Dialog).NameStartingWith("Allow resizing").ClassName("RootView")
	allowBtn := nodewith.Role(role.Button).Name("Allow").Ancestor(allowWin)
	if err := ui.WithTimeout(3 * time.Second).WaitUntilExists(allowWin)(ctx); err != nil {
		return nil
	}
	return ui.LeftClick(allowBtn)(ctx)
}
