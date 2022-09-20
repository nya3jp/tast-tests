// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package statusarea

import (
	"context"
	// "strconv"
	// "time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	// "chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DeviceState,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks the device state with status area elements",
		Contacts: []string{
			"kradtke@chromium.org",
			"cros-status-area-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

// DeviceState verifies that we can see device state on the status area.
func DeviceState(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	s.Log("Start testing status area device state")

	ui := uiauto.New(tconn)
	statusArea := nodewith.ClassName("UnifiedSystemTray").First()

	trayContainer := nodewith.ClassName("TrayContainer").Ancestor(statusArea)

	timeTrayView := nodewith.ClassName("TimeView").Ancestor(trayContainer)
	timeView := nodewith.ClassName("View").Ancestor(timeTrayView)
	timeLabel := nodewith.ClassName("Label").Ancestor(timeView)

	networkTrayView := nodewith.ClassName("NetworkTrayView").Ancestor(trayContainer)
	networkImage := nodewith.ClassName("ImageView").Ancestor(networkTrayView)

	powerTrayView := nodewith.ClassName("PowerTrayView").Ancestor(trayContainer)
	powerImage := nodewith.ClassName("ImageView").Ancestor(powerTrayView)

	if err := ui.WaitUntilExists(timeView)(ctx); err != nil {
		s.Fatal("Failed to find timeView: ", err)
	}

	if err := ui.WaitUntilExists(timeLabel)(ctx); err != nil {
		s.Fatal("Failed to find time item in status area: ", err)
	}

	if err := ui.WaitUntilExists(networkTrayView)(ctx); err != nil {
		s.Fatal("Failed to find networkTrayView in status area: ", err)
	}

	// don't know why it's not finding it
	if err := ui.WaitUntilExists(networkImage)(ctx); err != nil {
		s.Fatal("Failed to find networkImage in status area: ", err)
	}

	if err := ui.WaitUntilExists(powerImage)(ctx); err != nil {
		s.Fatal("Failed to find power item in status area: ", err)
	}
}
