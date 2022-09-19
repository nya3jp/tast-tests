// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diagnostics

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/diagnostics/utils"
	"chromiumos/tast/local/chrome/uiauto"
	da "chromiumos/tast/local/chrome/uiauto/diagnosticsapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Input,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Can successfully navigate to the Input page",
		Contacts: []string{
			"dpad@google.com",
			"ashleydp@google.com",
			"zentaro@google.com",
			"cros-peripherals@google.com",
		},
		Fixture:      "diagnosticsPrepForInputDiagnostics",
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// Input verifies that the Input page can be navigated to.
func Input(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*utils.FixtureData).Tconn

	// Since virtual keyboard with BUS_USB (0x03) doesn't work yet, use BUS_I2C (0x18).
	// See https://crrev.com/c/1407138 for more discussion.
	vkb, err := input.VirtualKeyboardWithBusType(ctx, 0x18)
	if err != nil {
		s.Fatal("Failed to create a virtual keyboard: ", err)
	}
	defer vkb.Close()

	// Find the Input navigation item and the keyboard list heading.
	const timeout = 10 * time.Second
	pollOpts := testing.PollOptions{Interval: time.Second, Timeout: timeout}
	ui := uiauto.New(tconn)
	inputTab := da.DxInput.Ancestor(da.DxRootNode)
	keyboardListHeading := da.DxKeyboardHeading.Ancestor(da.DxRootNode)
	if err := uiauto.Combine("find the keyboard list heading",
		ui.WithTimeout(timeout).WaitUntilExists(inputTab),
		ui.WithPollOpts(pollOpts).LeftClick(inputTab),
		ui.WithTimeout(timeout).WaitUntilExists(keyboardListHeading),
	)(ctx); err != nil {
		s.Fatal("Failed to find the keyboard list heading: ", err)
	}
}
