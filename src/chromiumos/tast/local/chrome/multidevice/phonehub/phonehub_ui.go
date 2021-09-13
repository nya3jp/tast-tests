// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package phonehub

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

// Enable enables Phone Hub from OS Settings using the UI. Assumes a connected device has already been paired.
func Enable(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) error {
	// Wait for connected devices to be available.
	_, err := ossettings.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to launch OS settings")
	}
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)
	connectedDeviceToggle := nodewith.Name("Connected phone features enable.").Role(role.ToggleButton)
	if err := ui.WaitUntilExists(connectedDeviceToggle)(ctx); err != nil {
		return errors.Wrap(err, "failed waiting for connected devices to be enabled")
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if info, err := ui.Info(ctx, connectedDeviceToggle); err != nil {
			return err
		} else if info.Checked != checked.True {
			return errors.New("connected devices not yet enabled")
		}
		return nil
	}, nil); err != nil {
		return errors.Wrap(err, "failed waiting for Android device to be connected")
	}

	// Turn on Phone Hub in the Connected devices subpage. The easiest way to get there is to reopen OS Settings on that specific page.
	phoneHubToggle := nodewith.Name("Phone Hub").Role(role.ToggleButton)
	_, err = ossettings.LaunchAtPageURL(ctx, tconn, cr, "multidevice/features", ui.WaitUntilExists(phoneHubToggle))
	if err != nil {
		return errors.Wrap(err, "failed to re-launch OS Settings to the multidevice feature page")
	}
	if info, err := ui.Info(ctx, phoneHubToggle); err != nil {
		errors.Wrap(err, "failed to get Phone Hub toggle button info")
	} else if info.Checked != checked.True {
		if err := ui.LeftClick(phoneHubToggle)(ctx); err != nil {
			errors.Wrap(err, "failed to toggle Phone Hub on")
		}
	}
	return nil
}
