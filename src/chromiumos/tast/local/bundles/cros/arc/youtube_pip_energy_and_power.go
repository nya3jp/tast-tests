// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

type youtubePIPEnergyAndPowerTestParams struct {
	tabletMode bool
	bigPIP     bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         YoutubePIPEnergyAndPower,
		Desc:         "Measures energy and power usage of ARC++ YouTube PIP",
		Contacts:     []string{"amusbach@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      15 * time.Minute,
		Vars:         []string{"arc.YoutubePIPEnergyAndPower.username", "arc.YoutubePIPEnergyAndPower.password"},
		Params: []testing.Param{{
			Name: "clamshell_small",
			Val:  youtubePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: false},
		}, {
			Name:              "tablet_small",
			Val:               youtubePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: false},
			ExtraSoftwareDeps: []string{"tablet_mode"},
		}, {
			Name: "clamshell_big",
			Val:  youtubePIPEnergyAndPowerTestParams{tabletMode: false, bigPIP: true},
		}, {
			Name:              "tablet_big",
			Val:               youtubePIPEnergyAndPowerTestParams{tabletMode: true, bigPIP: true},
			ExtraSoftwareDeps: []string{"tablet_mode"},
		}},
	})
}

func YoutubePIPEnergyAndPower(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(
		ctx,
		chrome.GAIALogin(),
		chrome.Auth(s.RequiredVar("arc.YoutubePIPEnergyAndPower.username"), s.RequiredVar("arc.YoutubePIPEnergyAndPower.password"), "gaia-id"),
		chrome.ARCSupported(),
		chrome.ExtraArgs("--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	params := s.Param().(youtubePIPEnergyAndPowerTestParams)
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, params.tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure tablet/clamshell mode: ", err)
	}
	defer cleanup(ctx)

	if err := ash.HideAllNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to hide notifications: ", err)
	}

	if err := audio.Mute(ctx); err != nil {
		s.Fatal("Failed to mute audio: ", err)
	}
	defer audio.Unmute(ctx)

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	var pointerController pointer.Controller
	var tew *input.TouchscreenEventWriter
	var stw *input.SingleTouchEventWriter
	if params.tabletMode {
		touchController, err := pointer.NewTouchController(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create touch controller: ", err)
		}
		pointerController = touchController
		tew = touchController.Touchscreen()
		stw = touchController.EventWriter()
	} else {
		pointerController = pointer.NewMouseController(tconn)
	}
	defer pointerController.Close()

	if err := optin.Perform(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to opt in to Play Store: ", err)
	}

	if err := optin.WaitForPlayStoreShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for Play Store: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	const ytAppPkgName = "com.google.android.youtube"
	if err := playstore.InstallApp(ctx, a, d, ytAppPkgName, 3); err != nil {
		s.Fatal("Failed to install ARC++ YouTube app: ", err)
	}

	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to close Play Store: ", err)
	}

	timeline, err := perf.NewTimeline(ctx, power.TestMetrics())
	if err != nil {
		s.Fatal("Failed to build metrics: ", err)
	}

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait for low CPU usage: ", err)
	}

	act, err := arc.NewActivity(a, ytAppPkgName, "com.google.android.apps.youtube.app.WatchWhileActivity")
	if err != nil {
		s.Fatal("Failed to create ARC++ YouTube app activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start ARC++ YouTube app: ", err)
	}
	defer act.Stop(ctx, tconn)

	if err := d.WaitForIdle(ctx, time.Minute); err != nil {
		s.Fatal("Failed to wait for ARC++ YouTube app to idle: ", err)
	}

	if err := d.Object(
		ui.ClassName("android.widget.ImageView"),
		ui.Description("Search"),
		ui.PackageName("com.google.android.youtube"),
	).Click(ctx); err != nil {
		s.Fatal("Failed to click Search: ", err)
	}

	if err := d.WaitForIdle(ctx, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for ARC++ YouTube app to idle: ", err)
	}

	if err := d.Object(
		ui.Text("Search YouTube"),
		ui.ClassName("android.widget.EditText"),
		ui.PackageName("com.google.android.youtube"),
	).SetText(ctx, "\"Top 100 Free Stock Videos 4K Rview and Download in Pixabay 12/2018\""); err != nil {
		s.Fatal("Failed to set search query: ", err)
	}

	if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
		s.Fatal("Failed to press Enter: ", err)
	}

	if err := d.WaitForIdle(ctx, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for ARC++ YouTube app to idle: ", err)
	}

	if err := d.Object(
		ui.Text("Search instead for \"Top 100 Free Stock Videos 4K Rview and Download in Pixabay 12/2018\""),
		ui.ClassName("android.widget.TextView"),
		ui.PackageName("com.google.android.youtube"),
	).Click(ctx); err != nil {
		s.Fatal("Failed to click to bypass spelling correction: ", err)
	}

	if err := d.WaitForIdle(ctx, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for ARC++ YouTube app to idle: ", err)
	}

	if err := d.Object(
		ui.ClassName("android.view.ViewGroup"),
		ui.DescriptionMatches("Top 100 Free Stock Videos 4K Rview and Download in Pixabay 12 2018 - 41 minutes - .+ - play video"),
		ui.PackageName("com.google.android.youtube"),
	).Click(ctx); err != nil {
		s.Fatal("Failed to click for video: ", err)
	}

	if err := d.WaitForIdle(ctx, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for ARC++ YouTube app to idle: ", err)
	}

	// Make sure that the video is playing and will go to PIP when we minimize the
	// window. Just waiting for the app to idle may not be enough.
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to wait five seconds: ", err)
	}

	if params.tabletMode {
		if err := ash.DragToShowHomescreen(ctx, tew.Width(), tew.Height(), stw, tconn); err != nil {
			s.Fatal("Failed to drag to show home launcher: ", err)
		}
	} else {
		if err := act.SetWindowState(ctx, arc.WindowStateMinimized); err != nil {
			s.Fatal("Failed to minimize ARC++ YouTube app: ", err)
		}
	}

	pipWindowFindParams := chromeui.FindParams{Name: "YouTube", ClassName: "RootView"}
	if err := chromeui.WaitUntilExists(ctx, tconn, pipWindowFindParams, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for PIP window: ", err)
	}

	pipWindow, err := chromeui.Find(ctx, tconn, pipWindowFindParams)
	if err != nil {
		s.Fatal("Failed to get PIP window: ", err)
	}
	defer pipWindow.Release(ctx)

	if !params.tabletMode {
		// Show the PIP video controls in preparation for the resizing drag.
		// Otherwise, that drag will move the PIP window instead of resizing it.
		if err := pointerController.Move(ctx, info.WorkArea.TopLeft(), pipWindow.Location.CenterPoint(), time.Second); err != nil {
			s.Fatal("Failed to move mouse: ", err)
		}
		if err := testing.Sleep(ctx, time.Second); err != nil {
			s.Fatal("Failed to wait a second: ", err)
		}
	}

	var resizeDragEnd coords.Point
	if params.bigPIP {
		resizeDragEnd = info.WorkArea.TopLeft()
	} else {
		resizeDragEnd = info.WorkArea.BottomRight().Sub(coords.NewPoint(1, 1))
	}
	if err := pointer.Drag(ctx, pointerController, pipWindow.Location.TopLeft(), resizeDragEnd, time.Second); err != nil {
		s.Fatal("Failed to resize PIP window: ", err)
	}

	if !params.tabletMode {
		// Ensure that the PIP window has no resize shadows showing.
		if err := pointerController.Move(ctx, resizeDragEnd, info.WorkArea.TopLeft().Add(coords.NewPoint(20, 20)), time.Second); err != nil {
			s.Fatal("Failed to move mouse: ", err)
		}
	}

	conn, err := cr.NewConn(ctx, "chrome://settings")
	if err != nil {
		s.Fatal("Failed to load chrome://settings: ", err)
	}
	defer conn.Close()

	if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for chrome://settings to achieve quiescence: ", err)
	}

	if err := timeline.Start(ctx); err != nil {
		s.Fatal("Failed to start metrics: ", err)
	}
	if err := timeline.StartRecording(ctx); err != nil {
		s.Fatal("Failed to start recording: ", err)
	}
	if err := testing.Sleep(ctx, time.Minute); err != nil {
		s.Fatal("Failed to wait a minute: ", err)
	}
	pv, err := timeline.StopRecording()
	if err != nil {
		s.Fatal("Error while recording metrics: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save perf data: ", err)
	}
}
