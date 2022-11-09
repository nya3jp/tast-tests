// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package instanttether

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/local/chrome/crossdevice"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Systemtray,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks enabling Instant Tether through Quick Settings",
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

func Systemtray(ctx context.Context, s *testing.State) {

	tconn := s.FixtValue().(*crossdevice.FixtData).TestConn
	ui := uiauto.New(tconn)

	// Open Quick Settings network menu.
	if err := quicksettings.NavigateToNetworkDetailedView(ctx, tconn, true); err != nil {
		s.Fatal("Failed to open Network Quick Settings menu: ", err)
	}

	// Determine the device's name to find it in the Quick Settings panel.
	deviceInfo, err := s.FixtValue().(*crossdevice.FixtData).AndroidDevice.GetAndroidAttributes(ctx)

	if (err) != nil {
		s.Fatal("Failed to retrieve information about paired phone")
	}

	deviceName := strings.Replace(deviceInfo.ModelName, "_", " ", -1)
	mobileNetworkView := nodewith.Role("button").NameRegex(regexp.MustCompile("Connect to .*" + deviceName)).Ancestor(quicksettings.NetworkDetailedViewRevamp)

	// Click on the button to connect to the mobile device.
	if err := uiauto.Combine("Find and click the mobile data entry",
		ui.WaitUntilExists(mobileNetworkView),
		ui.LeftClick(mobileNetworkView),
	)(ctx); err != nil {
		s.Fatal("Failed to click the mobile data button: ", err)
	}

	faillog.DumpUITree(ctx, s.OutDir(), tconn)

	// Ensure a connection has been established.
	detailsBtn := nodewith.Role("button").NameRegex(regexp.MustCompile("Open settings for .*" + deviceName)).Ancestor(quicksettings.NetworkDetailedViewRevamp)

	if err := ui.WaitUntilExists(detailsBtn)(ctx); err != nil {
		s.Fatal("Failed to find network detail button confirming Instant Tethering is connected: ", err)
	}
}
