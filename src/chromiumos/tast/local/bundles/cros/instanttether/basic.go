// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package instanttether

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/crossdevice"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Basic,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks basic Instant Tether functionality",
		Contacts: []string{
			"kyleshima@chromium.org",
			"chromeos-sw-engprod@google.com",
			"chromeos-cross-device-eng@google.com",
		},
		Attr:         []string{"group:cross-device-cellular"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				// Test connecting to Instant Tether exclusively through OS settings.
				Val: false,
			},
			{
				// Test connecting to Instant Tether with the "Wi-Fi available via phone"
				// notification that appears when tethering is available and CrOS has no
				// active network connections.
				Name: "connect_with_notification",
				Val:  true,
			},
		},
		Fixture: "crossdeviceOnboardedAllFeatures",
		Timeout: 5 * time.Minute,
	})
}

const tetherURL = "networks?type=Tether"

// Basic tests that Instant Tether can be enabled and the Chromebook can use the Android phone's mobile data connection.
// The test flow is slightly complicated due to the ADB-over-WiFi setup required by local tests in the lab.
// The key steps are:
// 1. Setup Instant Tether while all network connections (ethernet and wifi) are active.
//
// 2. Accept first-use dialogs/notifications on CrOS and Android if they appear.
//   - Thanks to 1, we can still use ADB-over-wifi to control the UI on the Android side.
//
// 3. Disconnect other networks to isolate the tethered connection and ensure it works.
//   - The default subtest disconnects ethernet and wifi and proceeds with verification.
//   - The connect_with_notification subtest disconnects ethernet, wifi, and also Instant Tether.
//     This triggers a "Wi-Fi available via phone" notification to appear, which can be used
//     to start the tethered connection with the phone. The subtest accepts that notification
//     to reconnect tethering, and then proceeds with verification.
func Basic(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*crossdevice.FixtData).Chrome
	tconn := s.FixtValue().(*crossdevice.FixtData).TestConn
	androidDevice := s.FixtValue().(*crossdevice.FixtData).AndroidDevice

	// Clear all notifications so we can easily surface the Instant Tether one.
	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to remove all notifications before starting the test: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*shill.EnableWaitTime+10*time.Second)
	defer cancel()

	tethered := false
	settings, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, tetherURL, func(context.Context) error { return nil })
	if err != nil {
		s.Fatal("Failed to launch OS settings to the tethered networks page: ", err)
	}

	// Defer disconnecting from the tethered network to clean up.
	disconnectBtn := nodewith.NameRegex(regexp.MustCompile("(?i)disconnect")).Role(role.Button)
	defer func() {
		if tethered {
			if err := settings.LeftClick(disconnectBtn)(cleanupCtx); err != nil {
				s.Log("Failed to click Instant Tether disconnect button: ", err)
			}
		}
	}()

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Initiate Instant Tethering while WiFi is still connected
	// so we still have ADB access to accept the mobile data provisioning notification.
	if err := uiauto.Combine("connecting to the mobile network",
		settings.LeftClick(nodewith.Role(role.Button).NameRegex(regexp.MustCompile("(?i)details")).ClassName("subpage-arrow")),
		settings.LeftClick(nodewith.NameRegex(regexp.MustCompile("(?i)connect")).Role(role.Button)),
	)(ctx); err != nil {
		s.Fatal("Failed to connect via OS Settings 'Connect' button: ", err)
	}

	// Accept first-use onboarding dialogs on both CrOS and Android.
	if err := handleFirstUseDialog(ctx, cr, settings, androidDevice); err != nil {
		s.Fatal("Failed to accept first-use dialog: ", err)
	}

	// Instant Tethering is now active and the Android's data connection is available to CrOS.
	tethered = true

	// Ensure the CrOS UI updates to reflect the tethered network's status.
	if err := settings.WaitUntilExists(nodewith.NameRegex(regexp.MustCompile(`(?i)instant tethering network, signal strength \d+%`)))(ctx); err != nil {
		s.Fatal("Failed to find text confirming Instant Tethering is connected: ", err)
	}

	// CrOS is currently connected to ethernet, WiFi, and has access to Android's mobile data.
	// To test that the tethered network can actually be used to access the internet on CrOS,
	// disable all other sources of internet by turning off the ethernet adapter and disconnecting
	// from the available WiFi network. Then tethering will be the only active network source,
	// and we can ensure CrOS still has internet access.
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}
	ethEnableFunc, err := manager.DisableTechnologyForTesting(ctx, shill.TechnologyEthernet)
	if err != nil {
		s.Fatal("Unable to disable ethernet: ", err)
	}
	defer ethEnableFunc(cleanupCtx)

	// If we want to test connecting Instant Tether with the notification that is triggered
	// when no networks are available, disconnect tethering here too so that notification will
	// appear and we can reconnect.
	useNotif := s.Param().(bool)
	if useNotif {
		// If we are testing
		if err := settings.LeftClick(disconnectBtn)(ctx); err != nil {
			s.Fatal("Failed to click Instant Tether disconnect button: ", err)
		}
	}

	// Just disconnect from the WiFi network instead of disabling it via shill (like we did for ethernet),
	// since the wifi adapter still needs to be on to use tethering.
	if err := crossdevice.DisconnectFromWifi(ctx); err != nil {
		s.Fatal("Failed to disconnect wifi: ", err)
	}
	defer crossdevice.ConnectToWifi(cleanupCtx)

	// At this point, CrOS is no longer connected to ethernet or WiFi.
	// In the default case, it is connected via Instant Tether. If testing
	// the notification, it is currently not connected to anything.
	if useNotif {
		// The notification informing the user that WiFi is available via phone
		// should now appear since there are no active network connections.
		if err := connectUsingNotification(ctx, tconn); err != nil {
			s.Fatal("Failed to connect with notification: ", err)
		}
	}

	// See if we have internet access over the tethered connection.
	if err := testing.Poll(ctx, func(context.Context) error {
		if err := networkAvailable(ctx, tconn, cr); err != nil {
			return errors.Wrap(err, "still waiting for network to be available")
		}
		return nil

	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Network not available after connecting with Instant Tethering")
	}
}

// networkAvailable checks if the network is available by navigating to a simple website.
func networkAvailable(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) error {
	c, err := cr.NewConn(ctx, "https://www.chromium.org/")
	if err != nil {
		return errors.Wrap(err, "failed to open browser")
	}
	defer c.Close()
	defer c.CloseTarget(ctx)
	ui := uiauto.New(tconn)
	if err := ui.WaitUntilExists(nodewith.Name("The Chromium Projects").Role(role.Heading))(ctx); err != nil {
		return errors.Wrap(err, "page did not load")
	}
	return nil
}

// connectUsingNotification connects to the phone's mobile data by accepting the Instant Tether notification.
func connectUsingNotification(ctx context.Context, tconn *chrome.TestConn) error {
	if _, err := ash.WaitForNotification(ctx, tconn, 30*time.Second,
		ash.WaitTitleContains("Wi-Fi available via phone"),
		ash.WaitMessageContains("Data connection available from your"),
	); err != nil {
		return errors.Wrap(err, "failed to wait for Instant Tether notification")
	}

	ui := uiauto.New(tconn)
	btn := nodewith.Role(role.Button).NameRegex(regexp.MustCompile("(?i)connect")).Ancestor(nodewith.Role(role.AlertDialog))
	if err := ui.LeftClick(btn)(ctx); err != nil {
		return errors.Wrap(err, "failed to click Instant Tether notification's connect button")
	}

	return nil
}

// handleFirstUseDialog accepts the first-use dialog for Instant Tether if it appears.
func handleFirstUseDialog(ctx context.Context, cr *chrome.Chrome, sconn *ossettings.OSSettings, ad *crossdevice.AndroidDevice) error {
	testing.ContextLog(ctx, "Waiting to see if first-use dialog is shown")
	firstUseText := nodewith.Role(role.StaticText).NameRegex(regexp.MustCompile("(?i)connect to new hotspot?"))
	if err := sconn.WithTimeout(10 * time.Second).WaitUntilExists(firstUseText)(ctx); err != nil {
		// If the first-use dialog doesn't appear, we don't need to do anything here.
		return nil
	}

	connectBtn := nodewith.Role(role.Button).NameRegex(regexp.MustCompile("(?i)connect")).Ancestor(nodewith.Role(role.Dialog))
	if err := sconn.LeftClick(connectBtn)(ctx); err != nil {
		return errors.Wrap(err, "failed to click first-use dialog's connect button")
	}

	// We need to accept a notification on the phone to initiate tethering for the first time.
	if err := ad.AcceptTetherNotification(ctx); err != nil {
		return errors.Wrap(err, "failed to accept tethering notification on the phone")
	}

	// Make sure OS settings is on the page we expect it to be on afterwards.
	if err := sconn.NavigateToPageURL(ctx, cr, tetherURL, func(context.Context) error { return nil }); err != nil {
		return errors.Wrap(err, "failed to return to Instant Tether settings page")
	}

	return nil
}
