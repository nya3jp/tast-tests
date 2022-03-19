// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nearbyshare is used to control Nearby Share functionality.
package nearbyshare

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
)

// JS for driving the Nearby Share subpage of OS settings.
const (
	// nearbySettingsSubpageJS is the locator for the settings-nearby-share-subpage element, which is the root element for accessing the subpage.
	nearbySettingsSubpageJS = `document.querySelector("os-settings-ui").shadowRoot` +
		`.querySelector("os-settings-main").shadowRoot` +
		`.querySelector("os-settings-page").shadowRoot` +
		`.querySelector("settings-multidevice-page").shadowRoot` +
		`.querySelector("settings-nearby-share-subpage")`
	showVisibilityDialogJS                              = nearbySettingsSubpageJS + `.showVisibilityDialog_ = true`
	contactVisibilityJS                                 = nearbySettingsSubpageJS + `.shadowRoot.querySelector("nearby-share-contact-visibility-dialog").shadowRoot.getElementById("contactVisibility")`
	onboardingSetUpJs                                   = nearbySettingsSubpageJS + `.shadowRoot.querySelector("#setUpButton").click()`
	onboardingPageJs                                    = nearbySettingsSubpageJS + `.shadowRoot.querySelector("nearby-share-receive-dialog").shadowRoot.querySelector("nearby-onboarding-one-page")`
	onboardingCompleteJs                                = onboardingPageJs + `.shadowRoot.querySelector("nearby-page-template").shadowRoot.querySelector("#actionButton").click()`
	onboardingGoToVisibilityPageJS                      = onboardingPageJs + `.shadowRoot.querySelector("#visibilityButton").click()`
	nearbyVisibilityPageJS                              = nearbySettingsSubpageJS + `.shadowRoot.querySelector("nearby-share-receive-dialog").shadowRoot.querySelector("nearby-visibility-page")`
	onboardingCompleteVisibilityPageJS                  = nearbyVisibilityPageJS + `.shadowRoot.querySelector("nearby-page-template").shadowRoot.querySelector("#actionButton").click()`
	toggleNearbyDevicesAreSharingNotificationJS         = nearbySettingsSubpageJS + `.shadowRoot.querySelector("#fastInitiationNotificationToggle").click()`
	getNearbyDevicesAreSharingNotificationToggleStateJS = nearbySettingsSubpageJS + `.shadowRoot.querySelector("#fastInitiationNotificationToggle").hasAttribute('checked')`
	contactsJS                                          = contactVisibilityJS + `.contacts`
	setAllowedConactsJS                                 = contactVisibilityJS + `.contactManager_.setAllowedContacts`
	settingsURL                                         = "chrome://os-settings/"
	nearbySettingsURL                                   = "multidevice/nearbyshare"
)

// NearbySettings is used to interact with the Nearby Share subpage of OS settings.
type NearbySettings struct {
	conn *chrome.Conn
}

// Close releases the resources associated with NearbySettings.
func (n *NearbySettings) Close(ctx context.Context) error {
	if err := n.conn.CloseTarget(ctx); err != nil {
		return errors.Wrap(err, "failed to close chrome://os-settings/ Chrome target")
	}
	if err := n.conn.Close(); err != nil {
		return errors.Wrap(err, "failed to close chrome://os-settings/ conn")
	}
	return nil
}

// nearbySettingsConn opens OS settings to the Nearby Share subpage and returns a Chrome conn to the page.
func nearbySettingsConn(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (*chrome.Conn, error) {
	_, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, nearbySettingsURL, func(context.Context) error { return nil })
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch OS Settings to Nearby Share page")
	}

	settingsConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(settingsURL+nearbySettingsURL))
	if err != nil {
		return nil, errors.Wrap(err, "failed to start Chrome session to OS settings")
	}
	if err = settingsConn.WaitForExpr(ctx, nearbySettingsSubpageJS); err != nil {
		return nil, errors.Wrap(err, "failed waiting for nearby subpage to load")
	}
	return settingsConn, nil
}

// LaunchNearbySettings launches OS settings to the Nearby Share subpage.
func LaunchNearbySettings(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (*NearbySettings, error) {
	settingsConn, err := nearbySettingsConn(ctx, tconn, cr)
	if err != nil {
		return nil, err
	}
	return &NearbySettings{settingsConn}, nil
}

// ToggleNearbyDeviceIsSharingNotification toggles the nearby device is trying to share notification setting on or off.
func ToggleNearbyDeviceIsSharingNotification(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, setChecked bool) error {
	nearbySettings, err := LaunchNearbySettings(ctx, tconn, cr)
	if err != nil {
		return errors.Wrap(err, "failed to launch nearby share settings")
	}

	var isChecked bool
	if err := nearbySettings.conn.Eval(ctx, getNearbyDevicesAreSharingNotificationToggleStateJS, &isChecked); err != nil {
		return errors.Wrap(err, "failed to get background scanning toggle state")
	}

	if isChecked == setChecked {
		return nil
	}

	if err := nearbySettings.conn.Eval(ctx, toggleNearbyDevicesAreSharingNotificationJS, nil); err != nil {
		return errors.Wrap(err, "failed to set background scanning toggle state")
	}
	return nil
}

// EnableNearbyShareInInitialOnboardingPage enables nearby share in the initial page of the onboarding workflow.
func EnableNearbyShareInInitialOnboardingPage(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) error {
	nearbySettings, err := LaunchNearbySettings(ctx, tconn, cr)
	if err != nil {
		return errors.Wrap(err, "failed to launch nearby share settings")
	}

	if err := nearbySettings.conn.Eval(ctx, onboardingSetUpJs, nil); err != nil {
		return errors.Wrap(err, "failed to use set up button to start onboarding workflow")
	}

	if err := nearbySettings.conn.WaitForExpr(ctx, onboardingPageJs); err != nil {
		return errors.Wrap(err, "failed waiting for onboarding view to load")
	}

	if err := nearbySettings.conn.Eval(ctx, onboardingCompleteJs, nil); err != nil {
		return errors.Wrap(err, "failed completing onboarding on initial page")
	}
	return nil
}

// EnableNearbyShareInVisibilitySelectionPage enables nearby share in the visibility selection page of the onboarding workflow.
func EnableNearbyShareInVisibilitySelectionPage(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) error {
	nearbySettings, err := LaunchNearbySettings(ctx, tconn, cr)
	if err != nil {
		return errors.Wrap(err, "failed to launch nearby share settings")
	}

	if err := nearbySettings.conn.Eval(ctx, onboardingSetUpJs, nil); err != nil {
		return errors.Wrap(err, "failed to use set up button to start onboarding workflow")
	}

	if err := nearbySettings.conn.WaitForExpr(ctx, onboardingPageJs); err != nil {
		return errors.Wrap(err, "failed waiting for onboarding view to load")
	}

	if err := nearbySettings.conn.Eval(ctx, onboardingGoToVisibilityPageJS, nil); err != nil {
		return errors.Wrap(err, "failed attempting to go to visibility selection page")
	}

	if err := nearbySettings.conn.WaitForExpr(ctx, nearbyVisibilityPageJS); err != nil {
		return errors.Wrap(err, "failed loading visibility selection page")
	}

	if err := nearbySettings.conn.Eval(ctx, onboardingCompleteVisibilityPageJS, nil); err != nil {
		return errors.Wrap(err, "failed completing onboarding on visibility selection page")
	}

	return nil
}

// GetNearbySettings connects to an existing OS settings Nearby Share subpage.
func GetNearbySettings(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (*NearbySettings, error) {
	settingsConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURLPrefix(settingsURL+nearbySettingsURL))
	if err != nil {
		return nil, errors.Wrap(err, "failed to start Chrome session to OS settings")
	}
	if err = settingsConn.WaitForExpr(ctx, nearbySettingsSubpageJS); err != nil {
		return nil, errors.Wrap(err, "failed waiting for nearby subpage to load")
	}
	return &NearbySettings{settingsConn}, nil
}

// ShowVisibilityDialog shows the visibility settings dialog, where we can choose a visibility setting and select which contacts to appear to.
func (n *NearbySettings) ShowVisibilityDialog(ctx context.Context) error {
	if err := n.conn.Eval(ctx, showVisibilityDialogJS, nil); err != nil {
		return errors.Wrap(err, "failed to run JS to show the visibility dialog")
	}
	if err := n.conn.WaitForExpr(ctx, contactVisibilityJS); err != nil {
		return errors.Wrap(err, "failed waiting for contactVisibility element to load")
	}
	// Wait for the contacts property to load.
	if err := n.conn.WaitForExpr(ctx, contactsJS); err != nil {
		return errors.Wrap(err, "failed waiting for contacts to load")
	}
	return nil
}

// WaitForOnboardingFlow waits for the Nearby Share setup page to open.
func (n *NearbySettings) WaitForOnboardingFlow(ctx context.Context) error {
	if err := n.conn.WaitForExpr(ctx, nearbySettingsSubpageJS); err != nil {
		return errors.Wrap(err, "failed waiting for onboarding page to load")
	}
	return nil
}

// findContactID returns the JS to evaluate to get the contact ID for a given email from the visibility dialog.
// The email is kept in the "description" field of the contact array.
func findContactID(email string) string {
	return fmt.Sprintf(contactsJS+`.find(c => c.description === %q).id`, email)
}

// SetAllowedContacts selects the contacts to will be able to see the device as a receiver.
func (n *NearbySettings) SetAllowedContacts(ctx context.Context, contacts ...string) error {
	if err := n.ShowVisibilityDialog(ctx); err != nil {
		return errors.Wrap(err, "failed to show the visibility dialog")
	}
	// First get the contact IDs corresponding to the input contact email addresses.
	var contactIDs []string
	for _, email := range contacts {
		var id string
		if err := n.conn.Eval(ctx, findContactID(email), &id); err != nil {
			return errors.Wrapf(err, "failed to find id for %v in the contact list", email)
		}
		contactIDs = append(contactIDs, id)
	}

	// The contact manager's `setAllowedContacts` function takes an array of strings as an input.
	var arg string
	if len(contactIDs) == 0 {
		arg = "([])"
	} else {
		arg = `(["` + strings.Join(contactIDs, `","`) + `"])`
	}

	if err := n.conn.Eval(ctx, setAllowedConactsJS+arg, nil); err != nil {
		return errors.Wrap(err, "failed to set allowed contacts")
	}
	return nil
}
