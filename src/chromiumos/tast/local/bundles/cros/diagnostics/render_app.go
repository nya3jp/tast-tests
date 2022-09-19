// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diagnostics

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/diagnostics/utils"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/diagnosticsapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RenderApp,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Diagnostics app launches and renders components",
		Contacts: []string{
			"ashleydp@google.com",
			"zentaro@google.com",
			"menghuan@google.com",
			"cros-peripherals@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "diagnosticsPrep",
	})
}

// RenderApp verifies launching an app from the launcher.
func RenderApp(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*utils.FixtureData).Tconn
	ui := uiauto.New(tconn).WithTimeout(20 * time.Second)

	// Verify cpu chart is drawn.
	if err := ui.WaitUntilExists(diagnosticsapp.DxCPUChart.Ancestor(
		diagnosticsapp.DxRootNode).First())(ctx); err != nil {
		s.Fatal("Failed to find CPU chart: ", err)
	}

	// Verify test routine button is rendered.
	if err := ui.WaitUntilExists(diagnosticsapp.DxCPUTestButton.Ancestor(
		diagnosticsapp.DxRootNode).First())(ctx); err != nil {
		s.Fatal("Failed to find cpu routine button: ", err)
	}

	if err := ui.WaitUntilExists(diagnosticsapp.DxMemoryTestButton.Ancestor(
		diagnosticsapp.DxRootNode).First())(ctx); err != nil {
		s.Fatal("Failed to find memory routine buttons: ", err)
	}

	// Open navigation if device is narrow view.
	if err := diagnosticsapp.ClickNavigationMenuButton(ctx, tconn); err != nil {
		s.Fatal("Could not click the menu button: ", err)
	}

	// Verify session log button is rendered.
	if err := ui.WaitUntilExists(diagnosticsapp.DxLogButton.Ancestor(
		diagnosticsapp.DxRootNode).First())(ctx); err != nil {
		s.Fatal("Failed to render log button: ", err)
	}
}
