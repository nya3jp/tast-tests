// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crossdevicesettings

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/testing"
)

const (
	// ConnectedDevicesSettingsURL is the path to the connected device settings page.
	ConnectedDevicesSettingsURL = "multidevice/features"
	// MultidevicePageJS is the JS locator for the multidevice settings page.
	MultidevicePageJS = `document.querySelector("os-settings-ui").shadowRoot` +
		`.querySelector("os-settings-main").shadowRoot` +
		`.querySelector("os-settings-page").shadowRoot` +
		`.querySelector("settings-multidevice-page")`
	// MultideviceSubpageJS is the JS locator for the multidevice settings subpage element.
	MultideviceSubpageJS = MultidevicePageJS + `.shadowRoot` +
		`.querySelector("settings-multidevice-subpage")`
	// ConnectedDeviceToggleVisibleJS is the JS locator for checking if the toggle to enable "Connected devices" is visible.
	ConnectedDeviceToggleVisibleJS = MultidevicePageJS + `.shouldShowToggle_()`
)

// WaitForConnectedDevice waits for the Android device to appear in the 'Connected device' section of OS Settings.
// Note: it can take up to 5 minutes for an Android device to successfully pair with the Chromebook.
// To account for this, the deadline of the passed in context should expire no sooner than 5 minutes from when this function is called.
func WaitForConnectedDevice(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) error {
	settings, err := ossettings.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to launch OS settings")
	}
	settingsConn, err := settings.ChromeConn(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to start Chrome session to OS settings")
	}
	defer settingsConn.Close()

	// Use JS to wait for a phone to be connected. If we are stuck waiting for the
	// "Connected devices" toggle to become visible (i.e. waiting for verification)
	// for more than 5 minutes, attempt to force a sync through the debug page.
	if ctxutil.DeadlineBefore(ctx, time.Now().Add(5*time.Minute)) {
		d, _ := ctx.Deadline()
		t := d.Sub(time.Now())
		return errors.Errorf("insufficient time remaining before the context reaches its deadline. need at least 5 minutes, only %v remain", t)
	}
	testing.ContextLog(ctx, "Waiting up to 5 minutes for the devices to be paired")
	if err := settingsConn.WaitForExpr(ctx, MultidevicePageJS); err != nil {
		return errors.Wrap(err, "failed waiting for \"Connected devices\" subpage to load")
	}
	if err := settingsConn.WaitForExprWithTimeout(ctx, ConnectedDeviceToggleVisibleJS, 5*time.Minute); err != nil {
		// Force a sync with the chrome://proximity-auth sync button and keep waiting.
		testing.ContextLog(ctx, "Devices did not pair within 5 minutes. Attempting to force a sync through the debug page")
		conn, err := cr.NewConn(ctx, "chrome://proximity-auth")
		if err != nil {
			return errors.Wrap(err, "failed to open chrome://proximity-auth")
		}
		defer conn.Close()
		syncBtn := `document.getElementById("force-device-sync")`
		if err := conn.WaitForExpr(ctx, syncBtn); err != nil {
			return errors.Wrap(err, "failed waiting for chrome://proximity-auth 'Sync' button to load")
		}
		if err := conn.Eval(ctx, syncBtn+`.click()`, nil); err != nil {
			return errors.Wrap(err, "failed to click chrome://proximity-auth 'Sync' button")
		}
		if err := settingsConn.WaitForExpr(ctx, ConnectedDeviceToggleVisibleJS); err != nil {
			return errors.Wrap(err, "'Connected devices' subpage not available after forcing device sync")
		}
	}
	return nil
}
