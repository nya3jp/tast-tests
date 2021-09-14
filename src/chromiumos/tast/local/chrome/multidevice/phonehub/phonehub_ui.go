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

// PhoneHubTray is the finder for the Phone Hub tray UI.
var PhoneHubTray = nodewith.Name("Phone Hub").ClassName("Widget")

// PhoneHubShelfIcon is the finder for the Phone Hub shelf icon.
var PhoneHubShelfIcon = nodewith.Name("Phone Hub").Role(role.Button).ClassName("PhoneHubTray")

// SilencePhonePod is the finder for Phone Hub's Silence Phone pod.
var SilencePhonePod = nodewith.NameContaining("Toggle Silence phone").Role(role.ToggleButton)

// Show opens Phone Hub if it's not already open.
func Show(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)
	if err := ui.Exists(PhoneHubTray)(ctx); err == nil { // Phone Hub already open
		return nil
	}
	if err := uiauto.Combine("click Phone Hub shelf icon and wait for it to open",
		ui.LeftClick(PhoneHubShelfIcon),
		ui.WaitUntilExists(PhoneHubTray),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to open Phone Hub")
	}
	return nil
}

// Hide hides Phone Hub if it's not already hidden.
func Hide(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)
	if err := ui.Exists(PhoneHubTray)(ctx); err != nil { // Phone Hub already hidden
		return nil
	}
	if err := uiauto.Combine("click Phone Hub shelf icon and wait for it to close",
		ui.LeftClick(PhoneHubShelfIcon),
		ui.WaitUntilGone(PhoneHubTray),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to close Phone Hub")
	}
	return nil
}

// PhoneSilenced returns true if the "Silence phone" pod is active, and false otherwise.
func PhoneSilenced(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	ui := uiauto.New(tconn)
	info, err := ui.Info(ctx, SilencePhonePod)
	if err != nil {
		return false, errors.Wrap(err, "failed to get node info for Silence Phone pod")
	}
	if info.Checked == checked.True {
		return true, nil
	}
	return false, nil
}

// WaitForPhoneSilenced waits for the Phone Silenced pod to be toggled on/off, since its state can be changed from the Android side.
func WaitForPhoneSilenced(ctx context.Context, tconn *chrome.TestConn, silenced bool, timeout time.Duration) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if curr, err := PhoneSilenced(ctx, tconn); err != nil {
			return err
		} else if curr != silenced {
			return errors.New("current Silence Phone status does not match the desired status")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "failed waiting for desired Do Not Disturb status")
	}
	return nil
}

// ToggleSilencePhonePod toggles Phone Hub's Silence Phone pod on/off.
func ToggleSilencePhonePod(ctx context.Context, tconn *chrome.TestConn, silence bool) error {
	ui := uiauto.New(tconn)
	curr, err := PhoneSilenced(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to check current Silence Phone setting")
	}
	if curr == silence {
		return nil
	}
	if err := ui.LeftClick(SilencePhonePod)(ctx); err != nil {
		return errors.Wrap(err, "failed to click Silence Phone pod")
	}
	return nil
}
