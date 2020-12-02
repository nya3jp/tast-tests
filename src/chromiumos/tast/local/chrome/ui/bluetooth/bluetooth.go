// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"regexp"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/ossettings"
	"chromiumos/tast/local/chrome/ui/quicksettings"
	"chromiumos/tast/testing"
)

// waitInterval defines the time duration between actions.
const waitInterval = 2 * time.Second

// Bluetooth implements bluetooth control.
type Bluetooth struct{}

// New creates a Bluetooth instance.  It also enables bluetooth feature through quick settings if disabled.
func New(ctx context.Context, tconn *chrome.TestConn) (*Bluetooth, error) {
	if err := quicksettings.ToggleSetting(ctx, tconn, quicksettings.SettingPodBluetooth, true); err != nil {
		return &Bluetooth{}, err
	}
	return &Bluetooth{}, nil
}

// Close disable bluetooth.
func (b *Bluetooth) Close(ctx context.Context, tconn *chrome.TestConn) error {
	testing.ContextLog(ctx, "bluetooth closed")
	if err := quicksettings.ToggleSetting(ctx, tconn, quicksettings.SettingPodBluetooth, false); err != nil {
		return err
	}
	return nil
}

// Connect connects to a Bluetooth device by the given name.
func (b *Bluetooth) Connect(ctx context.Context, tconn *chrome.TestConn, deviceName string) error {
	if deviceName == "" {
		return status.Error(codes.InvalidArgument, "device name is empty")
	}
	testing.ContextLogf(ctx, "bluetooth: connect %q via os settings", deviceName)

	if err := b.openSettings(ctx, tconn); err != nil {
		return err
	}
	defer apps.Close(ctx, tconn, apps.Settings.ID)

	params := newDeviceFindParams(deviceName)

	node, err := ui.FindWithTimeout(ctx, tconn, params, time.Second*30)
	if err != nil {
		return errors.Wrapf(err, "failed to click %q in os settings", deviceName)
	}
	defer node.Release(ctx)
	node.LeftClick(ctx)

	// wait the state to be "Connected"
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if connected, err := connectedState(ctx, tconn, deviceName); err != nil {
			return err
		} else if !connected {
			return errors.Wrap(err, "connection state is not connected")
		}
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
		return errors.Wrapf(err, "bluetooth device (%q) connect failed", deviceName)
	}
	return nil
}

// IsConnected checks if the named device is connected.
func (b *Bluetooth) IsConnected(ctx context.Context, tconn *chrome.TestConn, deviceName string) (bool, error) {
	if deviceName == "" {
		return false, status.Error(codes.InvalidArgument, "device name is empty")
	}
	testing.ContextLogf(ctx, "bluetooth: check %q connectivity", deviceName)

	if err := b.openSettings(ctx, tconn); err != nil {
		return false, err
	}
	defer apps.Close(ctx, tconn, apps.Settings.ID)

	// wait the state to be "Connected"
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if connected, err := connectedState(ctx, tconn, deviceName); err != nil {
			return err
		} else if !connected {
			return errors.Wrap(err, "connection state is not connected")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return false, errors.Wrapf(err, "failed to get bluetooth device status. deviceName=%q", deviceName)
	}
	return true, nil
}

// openSettings launches the os settings, navigating to the Bluetooth subpage.
func (b *Bluetooth) openSettings(ctx context.Context, tconn *chrome.TestConn) error {
	params := ui.FindParams{
		Role:      ui.RoleTypeButton,
		Name:      "Bluetooth",
		ClassName: "subpage-arrow",
	}
	if err := ossettings.LaunchAtPage(ctx, tconn, params); err != nil {
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
	node, err := ossettings.DescendantNodeWithTimeout(ctx, tconn, params, time.Second*5)
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
