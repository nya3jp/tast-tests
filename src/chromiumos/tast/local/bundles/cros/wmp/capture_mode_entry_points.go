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

	// Force stylus to be compatible with both the device and the display.
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--force-enable-stylus-tools", "--ash-enable-palette-on-all-displays"))
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Enable stylus tools to be accessed from the shelf. Needed with
	// the command line switch above to use the stylus tools capture
	// mode entry point.
	if err := tconn.Call(ctx, nil,
		`tast.promisify(chrome.settingsPrivate.setPref)`, "settings.enable_stylus_tools", true); err != nil {
		s.Fatal("Failed to set settings.enable_stylus_tools: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kw.Close()

	// Ensure that capture mode is not left active after the test completes.
	defer wmputils.EnsureCaptureModeActivated(tconn, false)(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	// Enter capture mode by clicking the associated feature pod in the
	// system tray bubble. We know it is in partial capture mode by the
	// presence of the user prompt telling users to select a capture
	// region.
	ac := uiauto.New(tconn)
	unifiedSystemTray := nodewith.HasClass("UnifiedSystemTray")
	captureModeBarView := nodewith.HasClass("CaptureModeBarView")
	captureModeFeaturePod := nodewith.HasClass("FeaturePodIconButton").Name("Screen capture")
	partialCaptureModeLabel := nodewith.Name("Drag to select an area to capture")
	if err := uiauto.Combine(
		"enter partial screen capture mode from system tray",
		ac.LeftClick(unifiedSystemTray),
		ac.LeftClick(captureModeFeaturePod),
		ac.WaitUntilExists(captureModeBarView),
		ac.WaitUntilExists(partialCaptureModeLabel),
		kw.AccelAction("Esc"),
	)(ctx); err != nil {
		s.Fatal("Failed to enter partial screen capture mode from system tray: ", err)
	}

	// Not all Chromebooks have the same layout for the function keys.
	layout, err := input.KeyboardTopRowLayout(ctx, kw)
	if err != nil {
		s.Fatal("Failed to get keyboard mapping: ", err)
	}

	// Enter partial capture mode by using the accelerator.
	if err := uiauto.Combine(
		"enter partial capture mode using accelerator",
		kw.AccelAction("Ctrl+Shift+"+layout.SelectTask),
		ac.WaitUntilExists(captureModeBarView),
		ac.WaitUntilExists(partialCaptureModeLabel),
		kw.AccelAction("Esc"),
	)(ctx); err != nil {
		s.Fatal("Failed to enter partial capture mode using accelerator: ", err)
	}

	// Ctrl + F4/F5 accelerator skips entering capture mode and instead
	// instantly takes a full-screen screenshot. Verify this by looking
	// for the screenshot notification.
	ashNotificationView := nodewith.HasClass("AshNotificationView").First()
	notificationLabel := nodewith.Name("Screenshot taken").Ancestor(ashNotificationView)
	if err := uiauto.Combine(
		"take a full screenshot using accelerator",
		kw.AccelAction("Ctrl+"+layout.SelectTask),
		ac.WaitUntilExists(notificationLabel),
	)(ctx); err != nil {
		s.Fatal("Failed to take a full screenshot using accelerator: ", err)
	}

	// Enter window capture mode using the accelerator. Unlike partial
	// and full capture mode, it doesn't have a user prompt, so we just
	// check that the user prompts for the other modes are not shown.
	// Devices reserve "Ctrl+Alt+(F2,F3,F4)" to go to command line. On
	// some devices with a certain layout, the shortcut here is
	// "Ctrl+Alt+F4". Skip this section if the select task key is F4.
	if layout.SelectTask == "F5" {
		fullCaptureModeLabel := nodewith.Name("Click anywhere to capture full screen")
		if err := uiauto.Combine(
			"enter window capture mode using accelerator",
			kw.AccelAction("Ctrl+Alt+"+layout.SelectTask),
			ac.WaitUntilExists(captureModeBarView),
			ac.EnsureGoneFor(fullCaptureModeLabel, 5*time.Second),
			ac.EnsureGoneFor(partialCaptureModeLabel, 5*time.Second),
			kw.AccelAction("Esc"),
		)(ctx); err != nil {
			s.Fatal("Failed to enter window capture mode using accelerator: ", err)
		}
	}

	// Use the stylus tool tray to enter capture mode.
	paletteTray := nodewith.HasClass("PaletteTray")
	paletteTrayBubble := nodewith.HasClass("TrayBubbleView").First()
	paletteTrayButton := nodewith.HasClass("HoverHighlightView").Name("Screen capture").Ancestor(paletteTrayBubble)
	if err := uiauto.Combine(
		"enter capture mode from stylus tools",
		ac.LeftClick(paletteTray),
		ac.LeftClick(paletteTrayButton),
		ac.WaitUntilExists(captureModeBarView),
	)(ctx); err != nil {
		s.Fatal("Failed to enter capture mode from stylus tools: ", err)
	}
}
