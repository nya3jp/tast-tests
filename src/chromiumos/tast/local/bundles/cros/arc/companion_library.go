// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CompanionLibrary,
		Desc:         "Test all ARC++ companion library",
		Contacts:     []string{"sstan@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"ArcCompanionLibDemo.apk"},
		Pre:          arc.Booted(),
		Timeout:      5 * time.Minute,
	})
}

func CompanionLibrary(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcCompanionLibDemo.apk"
		pkg = "org.chromium.arc.companionlibdemo"

		mainActivity = ".MainActivity"

		statusTextViewID                        = pkg + ":id/status_text_view"
		setCaptionButtonID                      = pkg + ":id/set_caption_buttons_visibility"
		getCaptionButtonID                      = pkg + ":id/get_caption_buttons_visibility"
		checkCaptionButtonMaximizeAndRestoreBox = pkg + ":id/caption_button_maximize_and_restore"
		checkCaptionButtonMinimizeBox           = pkg + ":id/caption_button_minimize"
		checkCaptionButtonGoBackBox             = pkg + ":id/caption_button_go_back"
		checkCaptionButtonCloseBox              = pkg + ":id/caption_button_close"
		checkCaptionButtonLegacyMenuBox         = pkg + ":id/caption_button_legacy_menu"
		getDeviceModeButtonID                   = pkg + ":id/get_device_mode_button"
		getWorkspaceInsetsButtonID              = pkg + ":id/get_workspace_insets"
		getDisplayTopologyButtonID              = pkg + ":id/get_display_topology"
		setTaskWindowStateButton                = pkg + ":id/set_task_window_state_button"
		getTaskWindowStateButton                = pkg + ":id/get_task_window_state_button"
	)

	cr := s.PreValue().(arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	// Restore tablet mode to its original state on exit.
	defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)

	// Force Chrome to be in clamshell mode, where windows are resizable.
	if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
		s.Fatal("Failed to disable tablet mode: ", err)
	}

	a := s.PreValue().(arc.PreData).ARC
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	act, err := arc.NewActivity(a, pkg, mainActivity)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed start Settings activity: ", err)
	}

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed to get device: ", err)
	}

	if err := act.WaitForIdle(ctx, time.Second); err != nil {
		s.Fatal("Failed to wait for idle activity: ", err)
	}

	must := func(err error) {
		if err != nil {
			s.Fatal(err) // NOLINT: arc/ui returns loggable errors
		}
	}

	lastString := func(lines []string) string {
		return lines[len(lines)-1]
	}

	testDeviceMode := func() {
		s.Log("Test Get Device Mode")
		getDeviceModeString := func() string {
			// Touch Get Device Mode Button
			if err := d.Object(ui.ID(getDeviceModeButtonID)).Click(ctx); err != nil {
				s.Fatal("Could not click the getDeviceModeButton")
			}
			must(act.WaitForIdle(ctx, time.Second))

			text, err := d.Object(ui.ID(statusTextViewID)).GetText(ctx)
			if err != nil {
				s.Fatal("Could not get text from status textview")
			}

			lastMessage := lastString(strings.Split(text, "\n"))
			if valid := strings.HasPrefix(lastMessage, "Device mode: "); valid != true {
				s.Fatalf("The format of message text is not valid: %s", lastMessage)
			}
			modeString := strings.TrimPrefix(lastMessage, "Device mode: ")
			return modeString
		}

		for _, test := range []struct {
			// systemMode represents current mode of system, true indecating Tablet mode, otherwise clamshell mode
			systemMode bool
			// modeStatus represents the expection of device mode string getting from companion library
			modeStatus string
		}{
			{systemMode: true, modeStatus: "TABLET"},
			{systemMode: false, modeStatus: "CLAMSHELL"},
		} {
			// Force Chrome to be in specific system mode
			if err := ash.SetTabletModeEnabled(ctx, tconn, test.systemMode); err != nil {
				s.Fatal("Failed to enable teblet mode: ", err)
			}
			must(act.WaitForIdle(ctx, time.Second))

			if modeFromAPI := getDeviceModeString(); modeFromAPI != test.modeStatus {
				s.Fatalf("Unexpected getDeviceMode result: got %s; want %s", modeFromAPI, test.modeStatus)
			}
		}
	}

	testDeviceMode()
}
