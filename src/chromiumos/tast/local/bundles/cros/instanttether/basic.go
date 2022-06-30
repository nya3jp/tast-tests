// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package instanttether

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
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
		// Enable once the lab is equipped to run tethering tests.
		// Attr:         []string{"group:cross-device"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "crossdeviceOnboarded",
		Timeout:      3 * time.Minute,
	})
}

const tetherURL = "networks?type=Tether"

// Basic tests that Instant Tether can be enabled and the Chromebook can use the Android phone's mobile data connection.
func Basic(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*crossdevice.FixtData).Chrome
	tconn := s.FixtValue().(*crossdevice.FixtData).TestConn

	// Clear all notifications so we can easily surface the Instant Tether one.
	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to remove all notifications before starting the test: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*shill.EnableWaitTime+10*time.Second)
	defer cancel()

	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	// Disable ethernet and wifi to ensure the tethered connection is being used.
	ethEnableFunc, err := manager.DisableTechnologyForTesting(ctx, shill.TechnologyEthernet)
	if err != nil {
		s.Fatal("Unable to disable ethernet: ", err)
	}
	defer ethEnableFunc(cleanupCtx)
	wifiEnableFunc, err := manager.DisableTechnologyForTesting(ctx, shill.TechnologyWifi)
	if err != nil {
		s.Fatal("Unable to disable ethernet: ", err)
	}
	defer wifiEnableFunc(cleanupCtx)

	// Confirm the network is unavailable before connecting with instant tethering.
	if networkAvailable(ctx) {
		s.Fatal("Network unexectedly available after disabling ethernet and wifi")
	}

	// Launch OS settings to the Instant Tether page, initiate tethering with the notification, and verify the Chromebook has network access.
	tethered := false
	settings, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, tetherURL, func(context.Context) error { return nil })
	if err != nil {
		s.Fatal("Failed to launch OS settings to the tethered networks page: ", err)
	}

	detailsBtn := nodewith.Role(role.Button).NameRegex(regexp.MustCompile("(?i)network 1 of 1.*connected"))
	defer func() {
		if tethered {
			if err := uiauto.Combine("disconnecting from the mobile network",
				settings.LeftClick(detailsBtn),
				settings.LeftClick(nodewith.NameRegex(regexp.MustCompile("(?i)disconnect")).Role(role.Button)),
			)(cleanupCtx); err != nil {
				s.Log("Failed to click Instant Tether disconnect button: ", err)
			}
		}
	}()

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	if err := connectUsingNotification(ctx, tconn); err != nil {
		s.Fatal("Failed to connect to Instant Tether via notification: ", err)
	}

	if err := handleFirstUseDialog(ctx, cr, settings); err != nil {
		s.Fatal("Failed to accept first-use dialog: ", err)
	}

	tethered = true

	if err := settings.WaitUntilExists(detailsBtn)(ctx); err != nil {
		s.Fatal("Failed to find network detail button confirming Instant Tethering is connected: ", err)
	}

	if !networkAvailable(ctx) {
		s.Fatal("Network not available after connecting with Instant Tethering")
	}
}

// networkAvailable returns whether or not the network is available.
func networkAvailable(ctx context.Context) bool {
	out, err := testexec.CommandContext(ctx, "ping", "-c", "3", "google.com").Output(testexec.DumpLogOnError)
	// An error is expected if we can't ping, so log it and return false.
	if err != nil {
		testing.ContextLog(ctx, "ping command failed: ", err)
		return false
	}
	return strings.Contains(string(out), "3 received")
}

// connectUsingNotification connects to the phone's mobile data by accepting the Instant Tether notification.
func connectUsingNotification(ctx context.Context, tconn *chrome.TestConn) error {
	if _, err := ash.WaitForNotification(ctx, tconn, 10*time.Second,
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
func handleFirstUseDialog(ctx context.Context, cr *chrome.Chrome, sconn *ossettings.OSSettings) error {
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

	// Make sure OS settings is on the page we expect it to be on afterwards.
	if err := sconn.NavigateToPageURL(ctx, cr, tetherURL, func(context.Context) error { return nil }); err != nil {
		return errors.Wrap(err, "failed to return to Instant Tether settings page")
	}

	return nil
}
