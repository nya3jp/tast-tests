// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package smartlock is for contrlling ChromeOS Smart Lock functionality.
package smartlock

import (
	"context"
//	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
//	"chromiumos/tast/local/chrome/uiauto"
//	"chromiumos/tast/local/chrome/uiauto/checked"
//	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
//	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
//	"chromiumos/tast/testing"
)

const (
	settingsURL                 = "chrome://os-settings/"
	connectedDevicesSettingsURL = "multidevice/features/"
	smartLockSettingsURL        = "smartLock"
	smartLockSubpage = `document.querySelector("os-settings-ui").shadowRoot` +
                `.querySelector("os-settings-main").shadowRoot` +
                `.querySelector("os-settings-page").shadowRoot` +
                `.querySelector("settings-multidevice-page").shadowRoot` +
                `.querySelector("settings-multidevice-smartlock-subpage").shadowRoot`
	smartLockUnlockButton = `.querySelector("multidevice-radio-button:nth-child(1)").shadowRoot` +
                `.querySelector("#button")`
	smartLockLoginButton = `.querySelector("multidevice-radio-button:nth-child(2)").shadowRoot` +
		`.querySelector("#button")`
	smartLockPasswordDialog = `.querySelector("#smartLockSignInPasswordPrompt")`

	multidevicePageJS           = `document.querySelector("os-settings-ui").shadowRoot` +
		`.querySelector("os-settings-main").shadowRoot` +
		`.querySelector("os-settings-page").shadowRoot` +
		`.querySelector("settings-multidevice-page")`
	multideviceSubpageJS = multidevicePageJS + `.shadowRoot` +
		`.querySelector("settings-multidevice-subpage")`
	smartLockToggleJS = multideviceSubpageJS +
		`.shadowRoot.getElementById("smartLockItem")` +
		`.shadowRoot.querySelector("settings-multidevice-feature-toggle")` +
		`.shadowRoot.getElementById("toggle")`
	smartLockSubpageItemJS = multideviceSubpageJS +
                `.shadowRoot.getElementById("smartLockItem")` +
		`.shadowRoot.getElementById("subpageButton")`
	featureCheckedJS               = `.checked`
	connectedDeviceToggleVisibleJS = multidevicePageJS + `.shouldShowToggle_()`
)

// OpenSmartLockSubpage opens the Smart Lock sub page in OS Settings
func OpenSmartLockSubpage(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (*ossettings.OSSettings, error) {
	// Open settings to the Smart Lock page.
        settings, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, connectedDevicesSettingsURL+smartLockSettingsURL, func(context.Context) error { return nil })
        if err != nil {
                return nil, errors.Wrap(err, "failed to launch OS Settings to the Smart Lock page")
        }
	return settings, nil
/*
        // Get connection to that page so we can execute javascript
        settingsConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(settingsURL+connectedDevicesSettingsURL+smartLockSettingsURL))
        if err != nil {
                return nil, errors.Wrap(err, "failed to start Chrome session to OS settings")
        }

        if err := settingsConn.WaitForExpr(ctx, smartLockSubpage+smartLockLoginButton); err != nil {
                return nil, errors.Wrap(err, "failed waiting for Smart Lock subpage to load")
        }
	return  settingsConn, nil
*/
}


// Enable enables Phone Hub from OS Settings using JS. Assumes a connected device has already been paired.
// Hide should be called afterwards to close the Phone Hub tray. It is left open here so callers can capture the UI state upon error if needed.
func EnableLogin(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, password string) error {
	kb, err := input.Keyboard(ctx)
        if err != nil {
                return errors.Wrap(err, "failed to get keyboard")
        }
        defer kb.Close()

	/*settingsConn, err := OpenSmartLockSubpage(ctx, tconn, cr)
	if err != nil {
		return errors.Wrap(err, "failed to open Smart lock sub page")
	}
	defer settingsConn.Close()
	if err := settingsConn.WaitForExpr(ctx, smartLockSubpage+smartLockLoginButton); err != nil {
		return errors.Wrap(err, "failed waiting for Smart Lock subpage to load")
	}
	// Toggle Login for Smart Lock with JS.
	if err := settingsConn.Eval(ctx, smartLockSubpage+smartLockLoginButton+`.click()`, nil); err != nil {
		return errors.Wrap(err, "failed to find click to enable Smart Lock login")
	}
	if err := settingsConn.WaitForExpr(ctx, smartLockSubpage+smartLockPasswordDialog); err != nil {
                return errors.Wrap(err, "failed waiting for Smart Lock subpage to load")
        }

	if err := kb.Type(ctx, password+"\n"); err != nil {
		return errors.Wrap(err, "entering password failed")
	}
	*/
	return nil
}

// EnableUnlock enables unlock-only for Smart Lock.
func EnableUnlock(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) error {
        /*settingsConn, err := OpenSmartLockSubpage(ctx, tconn, cr)
        if err != nil {
                return errors.Wrap(err, "failed to open Smart lock sub page")
        }
	
        defer settingsConn.Close()
        if err := settingsConn.WaitForExpr(ctx, smartLockSubpage+smartLockLoginButton); err != nil {
                return errors.Wrap(err, "failed waiting for Smart Lock subpage to load")
        }
        // Toggle Unlock button for Smart Lock with JS.
        if err := settingsConn.Eval(ctx, smartLockSubpage+smartLockUnlockButton+`.click()`, nil); err != nil {
                return errors.Wrap(err, "failed to find click to enable Smart Lock login")
        }
	*/
	return nil

}
