// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diagnostics

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/local/bundles/cros/diagnostics/utils"
	"chromiumos/tast/local/chrome/uiauto"
	da "chromiumos/tast/local/chrome/uiauto/diagnosticsapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         InputKeyboardConnectAndDisconnect,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Connect a virtual keyboard on the diagnostics input page and then disconnect it",
		Contacts: []string{
			"dpad@google.com",
			"jeff.lin@cienet.com",
			"xliu@cienet.com",
			"ashleydp@google.com",
			"zentaro@google.com",
			"cros-peripherals@google.com",
		},
		Fixture:      "diagnosticsPrepForInputDiagnostics",
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
	})
}

func InputKeyboardConnectAndDisconnect(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*utils.FixtureData).Tconn

	// Since virtual keyboard BUS_USB (0x03) doesn't work yet, use BUS_I2C (0x18).
	// See https://crrev.com/c/1407138 for more discussion.
	vkb, err := input.VirtualKeyboardWithBusType(ctx, 0x18)
	if err != nil {
		s.Fatal("Failed to create a virtual keyboard: ", err)
	}
	defer vkb.Close()

	ui := uiauto.New(tconn)
	inputTab := da.DxInput.Ancestor(da.DxRootNode)
	virtualKeyboard := da.DxVirtualKeyboardHeading
	if err := uiauto.Combine("check no virtual keyboard exists in input device list",
		ui.LeftClick(inputTab),
		ui.Gone(virtualKeyboard),
	)(ctx); err != nil {
		s.Fatal("Failed to check virtual keyboard: ", err)
	}

	disconnectKeyboard := func() action.Action {
		return func(ctx context.Context) error {
			return vkb.Close()
		}
	}

	if err := uiauto.Combine("verify virtual keyboard appears and disappears in the device list",
		ui.WaitUntilExists(virtualKeyboard),
		disconnectKeyboard(),
		ui.WaitUntilGone(virtualKeyboard),
	)(ctx); err != nil {
		s.Fatal("Failed to execute keyboard test: ", err)
	}
}
