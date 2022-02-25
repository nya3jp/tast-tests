// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package phonehub

import (
	"context"
	"strconv"
	"strings"
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

const (
	settingsURL                 = "chrome://os-settings/"
	connectedDevicesSettingsURL = "multidevice/features"
	multidevicePageJS           = `document.querySelector("os-settings-ui").shadowRoot` +
		`.querySelector("os-settings-main").shadowRoot` +
		`.querySelector("os-settings-page").shadowRoot` +
		`.querySelector("settings-multidevice-page")`
	multideviceSubpageJS = multidevicePageJS + `.shadowRoot` +
		`.querySelector("settings-multidevice-subpage")`
	phoneHubToggleJS = multideviceSubpageJS +
		`.shadowRoot.getElementById("phoneHubItem")` +
		`.shadowRoot.querySelector("settings-multidevice-feature-toggle")` +
		`.shadowRoot.getElementById("toggle")`
	recentPhotosToggleJS = multideviceSubpageJS +
		`.shadowRoot.getElementById("phoneHubCameraRollItem")` +
		`.shadowRoot.querySelector("settings-multidevice-feature-item")` +
		`.shadowRoot.querySelector("settings-multidevice-feature-toggle")` +
		`.shadowRoot.getElementById("toggle")`
	featureCheckedJS               = `.checked`
	connectedDeviceToggleVisibleJS = multidevicePageJS + `.shouldShowToggle_()`
)

// Enable enables Phone Hub from OS Settings using JS. Assumes a connected device has already been paired.
// Hide should be called afterwards to close the Phone Hub tray. It is left open here so callers can capture the UI state upon error if needed.
func Enable(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) error {
	_, err := ossettings.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to launch OS settings")
	}
	settingsConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(settingsURL))
	if err != nil {
		return errors.Wrap(err, "failed to start Chrome session to OS settings")
	}
	defer settingsConn.Close()

	// Use JS to wait for a phone to be connected.
	if err := settingsConn.WaitForExpr(ctx, multidevicePageJS); err != nil {
		return errors.Wrap(err, "failed waiting for \"Connected devices\" subpage to load")
	}
	if err := settingsConn.WaitForExpr(ctx, connectedDeviceToggleVisibleJS); err != nil {
		return errors.Wrap(err, "failed to wait for the \"Connected devices\" toggle is visible")
	}

	// Turn on Phone Hub in the "Connected devices" subpage. The easiest way to get there is to reopen OS Settings on that specific page.
	_, err = ossettings.LaunchAtPageURL(ctx, tconn, cr, connectedDevicesSettingsURL, func(context.Context) error { return nil })
	if err != nil {
		return errors.Wrap(err, "failed to re-launch OS Settings to the multidevice feature page")
	}

	// Toggle Phone Hub on with JS.
	if err := settingsConn.WaitForExpr(ctx, phoneHubToggleJS); err != nil {
		return errors.Wrap(err, "failed to find the Phone Hub toggle")
	}
	var enabled bool
	if err := settingsConn.Eval(ctx, phoneHubToggleJS+featureCheckedJS, &enabled); err != nil {
		return errors.Wrap(err, "failed to get Phone Hub toggle status")
	}
	if !enabled {
		if err := settingsConn.Eval(ctx, phoneHubToggleJS+`.click()`, nil); err != nil {
			return errors.Wrap(err, "failed to enable Phone Hub")
		}
	}
	// Check that the toggle was correctly enabled.
	if err := settingsConn.WaitForExpr(ctx, phoneHubToggleJS+featureCheckedJS+`===true`); err != nil {
		return errors.Wrap(err, "failed to toggle Phone Hub on using JS")
	}
	// Phone Hub is still not immediately ready to use after toggling it on from OS Settings,
	// since it takes a short amount of time for it to connect to the phone and display anything.
	// Wait for it to become usable by checking for the existence of a settings pod.
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)
	if err := Show(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to show Phone Hub")
	}
	if err := ui.WaitUntilExists(phoneHubSettingPod.First())(ctx); err != nil {
		return errors.Wrap(err, "failed to find a Phone Hub setting pod")
	}

	return nil
}

// PhoneHubTray is the finder for the Phone Hub tray UI.
var PhoneHubTray = nodewith.Name("Phone Hub").ClassName("Widget")

// PhoneHubShelfIcon is the finder for the Phone Hub shelf icon.
var PhoneHubShelfIcon = nodewith.Name("Phone Hub").Role(role.Button).ClassName("PhoneHubTray")

// phoneHubSettingPod is the base UI finder for the individual setting pods.
var phoneHubSettingPod = nodewith.Ancestor(PhoneHubTray).Role(role.ToggleButton)

// SilencePhonePod is the finder for Phone Hub's Silence Phone pod.
var SilencePhonePod = phoneHubSettingPod.NameContaining("Toggle Silence phone")

// Show opens Phone Hub if it's not already open.
func Show(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)
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
		wanted := "off"
		if silenced {
			wanted = "on"
		}
		return errors.Wrapf(err, "failed waiting for Silence Phone to be turned %v", wanted)
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
	if err := WaitForPhoneSilenced(ctx, tconn, silence, 15*time.Second); err != nil {
		return errors.Wrap(err, "failed waiting for Silence Phone pod to be toggled after clicking")
	}
	return nil
}

// FindRecentPhotosOptInButton returns a finder which locates the opt-in button for the Recent Photos feature.
func FindRecentPhotosOptInButton() *nodewith.Finder {
	return nodewith.Ancestor(nodewith.ClassName("CameraRollView")).Name("Turn on").Role(role.Button)
}

// OptInRecentPhotos enables the Recent Photos feature by clicking on the opt-in button displayed in the Phone Hub bubble.
func OptInRecentPhotos(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)
	if err := ui.LeftClick(FindRecentPhotosOptInButton())(ctx); err != nil {
		return errors.Wrap(err, "failed to click on the Recent Photos opt-in button")
	}
	return nil
}

// DownloadMostRecentPhoto downloads the first photo shown in Phone Hub's Recent Photos section to Tote.
func DownloadMostRecentPhoto(ctx context.Context, tconn *chrome.TestConn) error {
	mostRecentPhotoThumbnail := nodewith.Ancestor(PhoneHubTray).ClassName("CameraRollThumbnail").First()
	// It might take some time to receive and display the recent photo thumbnails.
	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)
	if err := uiauto.Combine("download the first photo displayed in the Recent Photos section",
		ui.LeftClick(mostRecentPhotoThumbnail),
		ui.LeftClick(nodewith.Role(role.MenuItem).Name("Download")),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to download recent photo")
	}
	return nil
}

// ToggleRecentPhotosSetting toggles the Recent Photos setting using JS. This assumes that a connected device has already been paired.
func ToggleRecentPhotosSetting(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, enable bool) error {
	_, err := ossettings.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to launch OS settings")
	}
	settingsConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(settingsURL))
	if err != nil {
		return errors.Wrap(err, "failed to start Chrome session to OS settings")
	}
	defer settingsConn.Close()

	_, err = ossettings.LaunchAtPageURL(ctx, tconn, cr, connectedDevicesSettingsURL, func(context.Context) error { return nil })
	if err != nil {
		return errors.Wrap(err, "failed to re-launch OS Settings to the multidevice feature page")
	}

	if err := settingsConn.WaitForExpr(ctx, recentPhotosToggleJS); err != nil {
		return errors.Wrap(err, "failed to find the Recent Photos toggle")
	}
	var isEnabled bool
	if err := settingsConn.Eval(ctx, recentPhotosToggleJS+featureCheckedJS, &isEnabled); err != nil {
		return errors.Wrap(err, "failed to get Recent Photos toggle status")
	}
	if isEnabled != enable {
		if err := settingsConn.Eval(ctx, recentPhotosToggleJS+`.click()`, nil); err != nil {
			return errors.Wrap(err, "failed to click on Recent Photos toggle")
		}
	}
	if err := settingsConn.WaitForExpr(ctx, recentPhotosToggleJS+featureCheckedJS+`===`+strconv.FormatBool(enable)); err != nil {
		return errors.Wrapf(err, "failed to toggle Recent Photos to %v using JS", strconv.FormatBool(enable))
	}

	return nil
}

// RecentTabChipFinder is the finder for Phone Hub "Recent Chrome tab" chips.
var RecentTabChipFinder = nodewith.Role(role.Button).HasClass("ContinueBrowsingChip")

// RecentTabChip represents one of Phone Hub's "Recent Chrome tab" chips.
type RecentTabChip struct {
	URL    string
	Finder *nodewith.Finder
}

// RecentTabChips returns all of the "Recent Chrome tab" chips currently displayed in Phone Hub.
func RecentTabChips(ctx context.Context, tconn *chrome.TestConn) ([]RecentTabChip, error) {
	ui := uiauto.New(tconn)
	if err := ui.WaitUntilExists(RecentTabChipFinder.First())(ctx); err != nil {
		return nil, errors.Wrap(err, "no recent tab chips found")
	}
	nodes, err := ui.NodesInfo(ctx, RecentTabChipFinder)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get UI node info about recent tab chips")
	}

	var chips []RecentTabChip
	for _, node := range nodes {
		// Extract the URL from the Name attribute, which looks like:
		//   name=Browser tab 1 of 1. Blah, https://www.blah.org/
		name := node.Name
		parts := strings.Split(name, " ")
		url := parts[len(parts)-1]
		chips = append(chips, RecentTabChip{URL: url, Finder: RecentTabChipFinder.Name(name)})
	}
	return chips, nil
}
