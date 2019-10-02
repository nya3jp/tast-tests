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

func CompanionLibrary(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcCompanionLibDemo.apk"
		pkg = "org.chromium.arc.companionlibdemo"

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
	const (
		pkg                   = "org.chromium.arc.companionlibdemo"
		statusTextViewID      = pkg + ":id/status_text_view"
		getDeviceModeButtonID = pkg + ":id/get_device_mode_button"
	)
	s.Log("Test Get Device Mode")

	getLastString := func(lines []string) string {
		return lines[len(lines)-1]
	}

	getTextViewContent := func() []string {
		text, err := d.Object(ui.ID(statusTextViewID)).GetText(ctx)
		if err != nil {
			s.Fatal("StatusTextView not ready yet: ", err)
		}
		return strings.Split(text, "\n")
	}

	getDeviceModeString := func() string {
		// Check new message by compare the textview content length
		originalLength := len(getTextViewContent())
		if err := d.Object(ui.ID(getDeviceModeButtonID)).Click(ctx); err != nil {
			s.Fatal("Could not click the getDeviceMode button: ", err)
		}

		err := testing.Poll(ctx, func(ctx context.Context) error {
			if len(getTextViewContent()) == originalLength {
				return errors.New("textview still waiting update")
			}
			return nil
		}, &testing.PollOptions{Timeout: 2 * time.Second})
		if err != nil {
			s.Fatal("Error get new line in status text view: ", err)
		}
		lastMessage := getLastString(getTextViewContent())
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
		err := testing.Poll(ctx, func(ctx context.Context) error {
			if modeFromAPI := getDeviceModeString(); modeFromAPI != test.modeStatus {
				errors.Errorf("unexpected getDeviceMode result: got %s; want %s", modeFromAPI, test.modeStatus)
			}
			return nil
		}, &testing.PollOptions{Timeout: 4 * time.Second})
		if err != nil {
			s.Fatal("Error while get device mode: ", err)
		}
	}
	return nil
}
