// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CompanionLibrary,
		Desc:         "Test all ARC++ companion library",
		Contacts:     []string{"sstan@google.com", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{"ArcCompanionLibDemo.apk"},
		Pre:          arc.Booted(),
		Timeout:      5 * time.Minute,
	})
}

const pkg = "org.chromium.arc.companionlibdemo"

func CompanionLibrary(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcCompanionLibDemo.apk"

		mainActivity = ".MainActivity"
	)

	must := func(err error) {
		if err != nil {
			s.Fatal(err) // NOLINT: arc/ui returns loggable errors
		}
	}

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
	defer d.Close()

	if err := act.WaitForIdle(ctx, time.Second); err != nil {
		s.Fatal("Failed to wait for idle activity: ", err)
	}

	type testFunc func(context.Context, *chrome.Conn, *arc.Activity, *ui.Device, *testing.State) error
	for _, test := range []struct {
		name string
		fn   testFunc
	}{
		{"Get Device Mode", testDeviceMode},
	} {
		s.Logf("Running %q", test.name)
		must(act.Start(ctx))
		must(act.WaitForIdle(ctx, time.Second))
		if err := test.fn(ctx, tconn, act, d, s); err != nil {
			s.Errorf("%s test failed: %v", test.name, err)
		}
		must(act.Stop(ctx))
	}

}

func testDeviceMode(ctx context.Context, tconn *chrome.Conn, act *arc.Activity, d *ui.Device, s *testing.State) error {
	const getDeviceModeButtonID = pkg + ":id/get_device_mode_button"

	getLastString := func(lines []string) string {
		return lines[len(lines)-1]
	}

	getDeviceModeString := func() string {
		// Each output in ArcCompanionLib Demo will gengerate a line in the TextView
		// Get the new message from the latest line
		var originalLength int
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			// UI component not always stable, especially in first time, so Poll here
			originalLength = len(getTextViewContent(ctx, d))
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			s.Fatal("Could not get text in textview: ", err)
		}

		if err := d.Object(ui.ID(getDeviceModeButtonID)).Click(ctx); err != nil {
			s.Fatal("Could not click the getDeviceMode button: ", err)
		}

		err := testing.Poll(ctx, func(ctx context.Context) error {
			if len(getTextViewContent(ctx, d)) == originalLength {
				return errors.New("textview still waiting update")
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second})
		if err != nil {
			s.Fatal("Error get new line in status text view: ", err)
		}

		lastMessage := getLastString(getTextViewContent(ctx, d))
		if valid := strings.HasPrefix(lastMessage, "Device mode: "); valid != true {
			s.Fatalf("The format of message text is not valid: %s", lastMessage)
		}
		modeString := strings.TrimPrefix(lastMessage, "Device mode: ")
		return modeString
	}

	for _, test := range []struct {
		// isTabletMode represents current mode of system which is Tablet mode or clamshell mode
		isTabletMode bool
		// modeStatus represents the expection of device mode string getting from companion library
		modeStatus string
	}{
		{isTabletMode: true, modeStatus: "TABLET"},
		{isTabletMode: false, modeStatus: "CLAMSHELL"},
	} {
		// Force Chrome to be in specific system mode
		if err := ash.SetTabletModeEnabled(ctx, tconn, test.isTabletMode); err != nil {
			s.Fatal("Failed to set the system mode: ", err)
		}
		if modeFromAPI := getDeviceModeString(); modeFromAPI != test.modeStatus {
			s.Fatalf("Unexpected getDeviceMode result: got %s; want %s", modeFromAPI, test.modeStatus)
		}
	}
	return nil
}

func getTextViewContent(ctx context.Context, d *ui.Device) []string {
	const statusTextViewID = pkg + ":id/status_text_view"
	text, err := d.Object(ui.ID(statusTextViewID)).GetText(ctx)
	if err != nil {
		// Use errors because it not always success when get object, poll is necessary
		errors.Errorf("StatusTextView not ready yet: %s", err)
	}
	return strings.Split(text, "\n")
}
