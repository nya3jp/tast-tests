// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

// Connect connects to a Bluetooth device by the given name.
func Connect(ctx context.Context, tconn *chrome.TestConn, deviceName string) error {
	if deviceName == "" {
		return errors.New("device name is empty")
	}

	if err := quicksettings.ToggleSetting(ctx, tconn, quicksettings.SettingPodBluetooth, true); err != nil {
		return err
	}

	testing.ContextLogf(ctx, "Connecting %q bluetooth device via os settings", deviceName)

	if err := openSettings(ctx, tconn); err != nil {
		return err
	}
	defer apps.Close(ctx, tconn, apps.Settings.ID)

	if connected, err := connectedState(ctx, tconn, deviceName); err != nil {
		return err
	} else if connected {
		testing.ContextLogf(ctx, "%q bluetooth device is connected", deviceName)
		return nil
	}

	params := newDeviceFindParams(deviceName)
	node, err := ui.FindWithTimeout(ctx, tconn, params, time.Second*30)
	if err != nil {
		return errors.Wrapf(err, "failed to find %q in os settings", deviceName)
	}
	defer node.Release(ctx)
	node.StableLeftClick(ctx, &testing.PollOptions{Interval: time.Second, Timeout: time.Second * 10})

	// wait the state to be "Connected"
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if connected, err := connectedState(ctx, tconn, deviceName); err != nil {
			return testing.PollBreak(err)
		} else if !connected {
			return errors.Wrapf(err, "%q is not connected", deviceName)
		}
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
		return errors.Wrapf(err, "bluetooth device (%q) connect failed", deviceName)
	}
	return nil
}

// IsDeviceExist checks if the specified device exists as bluetooth device.
func IsDeviceExist(ctx context.Context, tconn *chrome.TestConn, deviceName string) bool {
	params := newDeviceFindParams(deviceName)
	if err := ui.WaitUntilExists(ctx, tconn, params, time.Second*30); err != nil {
		return false
	}
	return true
}

// Disable disables bluetooth.
func Disable(ctx context.Context, tconn *chrome.TestConn) error {
	testing.ContextLog(ctx, "disable bluetooth")
	if err := quicksettings.ToggleSetting(ctx, tconn, quicksettings.SettingPodBluetooth, false); err != nil {
		return err
	}
	return nil
}

// IsConnected checks if the named device is connected.
func IsConnected(ctx context.Context, tconn *chrome.TestConn, deviceName string) (bool, error) {
	if deviceName == "" {
		return false, errors.New("device name is empty")
	}

	if currentState, err := quicksettings.SettingEnabled(ctx, tconn, quicksettings.SettingPodBluetooth); err != nil {
		return false, err
	} else if !currentState {
		return false, nil
	}

	testing.ContextLogf(ctx, "Checking the connectivity of %q bluetooth device", deviceName)

	if err := openSettings(ctx, tconn); err != nil {
		return false, err
	}
	defer apps.Close(ctx, tconn, apps.Settings.ID)
	connected, err := connectedState(ctx, tconn, deviceName)
	if err != nil {
		return false, err
	}
	return connected, nil
}

// openSettings launches the os settings and navigates to the Bluetooth subpage.
func openSettings(ctx context.Context, tconn *chrome.TestConn) error {
	if _, err := ossettings.LaunchAtPage(ctx, tconn, nodewith.Name("Bluetooth").Role(role.Button)); err != nil {
		return errors.Wrap(err, "failed to launch Bluetooth page")
	}
	return nil
}

func newDeviceFindParams(name string) ui.FindParams {
	pattern := regexp.QuoteMeta(name)
	return ui.FindParams{
		Role:       ui.RoleTypeButton,
		ClassName:  "list-item",
		Attributes: map[string]interface{}{"name": regexp.MustCompile(pattern)},
	}
}

// connectedState returns the connection state of a bluetooth device from the settings window.
func connectedState(ctx context.Context, tconn *chrome.TestConn, deviceName string) (bool, error) {
	params := newDeviceFindParams(deviceName)
	node, err := ui.FindWithTimeout(ctx, tconn, params, time.Second*5)
	if err != nil {
		return false, err
	}
	defer node.Release(ctx)

	label, err := node.Descendant(ctx, ui.FindParams{
		Attributes: map[string]interface{}{"name": regexp.MustCompile("[Cc]onnect")},
	})
	if err != nil {
		return false, err
	}
	defer label.Release(ctx)
	return label.Name == "Connected", nil
}
