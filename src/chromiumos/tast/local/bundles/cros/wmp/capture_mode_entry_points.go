// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/wmp/wmputils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CaptureModeEntryPoints,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests entering capture mode with a variety of entry points",
		Contacts: []string{
			"sammiequon@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

func CaptureModeEntryPoints(ctx context.Context, s *testing.State) {
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Force stylus to be compatible with the device.
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--force-enable-stylus-tools"))
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Enable stylus tools to be accessed from the shelf. Needed with the command line switch above to use the stylus
	// tools capture mode entry point.
	if err := tconn.Call(ctx, nil,
		`tast.promisify(chrome.settingsPrivate.setPref)`, "settings.enable_stylus_tools", true); err != nil {
		s.Fatal("Failed to set settings.enable_stylus_tools: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	ac := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}

	// Ensure case exit screen capture mode.
	defer wmputils.EnsureCaptureModeActivated(tconn, false)(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	// Enter capture mode by clicking the associated feature pod in the system tray bubble. We know it is in partial
	// capture mode by the presence of the user prompt telling users to select a capture region.
	unifiedSystemTray := nodewith.ClassName("UnifiedSystemTray")
	captureModeBarView := nodewith.ClassName("CaptureModeBarView")
	captureModeFeaturePod := nodewith.ClassName("FeaturePodIconButton").Name("Screen capture")
	partialCaptureModeLabel := nodewith.ClassName("Label").Name("Drag to select an area to capture")
	if err := uiauto.Combine(
		"enter partial screen capture mode from system tray",
		ac.LeftClick(unifiedSystemTray),
		ac.WaitUntilExists(captureModeFeaturePod),
		ac.LeftClick(captureModeFeaturePod),
		ac.WaitUntilExists(captureModeBarView),
		ac.WaitUntilExists(partialCaptureModeLabel),
		kb.AccelAction("esc"),
	)(ctx); err != nil {
		s.Fatal("Failed to enter partial screen capture mode from system tray: ", err)
	}

	// Enter partial capture mode by using the accelerator.
	if err := uiauto.Combine(
		"enter partial capture mode using accelerator",
		kb.AccelAction("Ctrl+Shift+F5"),
		ac.WaitUntilExists(captureModeBarView),
		ac.WaitUntilExists(partialCaptureModeLabel),
		kb.AccelAction("esc"),
	)(ctx); err != nil {
		s.Fatal("Failed to enter partial capture mode using accelerator: ", err)
	}

	// Ctrl + F5 accelerator skips entering capture mode and instead instantly takes a full screen screenshot. Verify
	// this by looking for the screenshot notification.
	ashNotificationView := nodewith.ClassName("AshNotificationView")
	notificationLabel := nodewith.ClassName("Label").Name("Screenshot taken").Ancestor(ashNotificationView)
	if err := uiauto.Combine(
		"take a full screenshot using accelerator",
		kb.AccelAction("Ctrl+F5"),
		ac.WaitUntilExists(notificationLabel),
		kb.AccelAction("esc"),
	)(ctx); err != nil {
		s.Fatal("Failed to take a full screenshot using accelerator: ", err)
	}

	// Enter window capture mode using the accelerator. Unlike partial and full capture mode, it doesn't have a user
	// prompt, so we just check that the user prompts for the other modes are not shown.
	fullCaptureModeLabel := nodewith.ClassName("Label").Name("Click anywhere to capture full screen")
	if err := uiauto.Combine(
		"enter window capture mode using accelerator",
		kb.AccelAction("Ctrl+Alt+F5"),
		ac.WaitUntilExists(captureModeBarView),
		ac.WaitUntilGone(fullCaptureModeLabel),
		ac.WaitUntilGone(partialCaptureModeLabel),
		kb.AccelAction("esc"),
	)(ctx); err != nil {
		s.Fatal("Failed to enter window capture mode using accelerator: ", err)
	}

	paletteTray := nodewith.ClassName("PaletteTray")
	paletteTrayBubble := nodewith.ClassName("TrayBubbleView").First()
	paletteTrayButton := nodewith.ClassName("HoverHighlightView").Name("Screen capture").Ancestor(paletteTrayBubble)
	if err := uiauto.Combine(
		"enter capture mode from stylus tools",
		ac.LeftClick(paletteTray),
		ac.WaitUntilExists(paletteTrayButton),
		ac.LeftClick(paletteTrayButton),
		ac.WaitUntilExists(captureModeBarView),
	)(ctx); err != nil {
		s.Fatal("Failed to enter capture mode from stylus tools: ", err)
	}
}
