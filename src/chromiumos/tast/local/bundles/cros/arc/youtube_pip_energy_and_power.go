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
	"chromiumos/tast/local/bundles/cros/arc/pipresize"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

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
			Name: "small",
			Val:  false,
		}, {
			Name: "big",
			Val:  true,
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

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(ctx)

	if err := ash.HideAllNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to hide notifications: ", err)
	}

	if err := audio.Mute(ctx); err != nil {
		s.Fatal("Failed to mute audio: ", err)
	}
	defer audio.Unmute(ctx)

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

	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard event writer: ", err)
	}
	defer kw.Close()

	timeline, err := perf.NewTimeline(ctx, power.TestMetrics())
	if err != nil {
		s.Fatal("Failed to build metrics: ", err)
	}

	if _, err := power.WaitUntilCPUCoolDown(ctx, power.CoolDownPreserveUI); err != nil {
		s.Fatal("Failed to wait for CPU to cool down: ", err)
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

	if err := act.SetWindowState(ctx, tconn, arc.WindowStateMinimized); err != nil {
		s.Fatal("Failed to minimize ARC++ YouTube app: ", err)
	}

	if err := pipresize.WaitForPIPAndSetSize(ctx, tconn, d, s.Param().(bool)); err != nil {
		s.Fatal("Failed to wait for PIP window and set size: ", err)
	}

	conn, err := cr.NewConn(ctx, "chrome://settings")
	if err != nil {
		s.Fatal("Failed to load chrome://settings: ", err)
	}
	defer conn.Close()

	if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for chrome://settings to achieve quiescence: ", err)
	}

	// Tab away from the search box of chrome://settings, so that
	// there will be no blinking cursor.
	if err := kw.Accel(ctx, "Tab"); err != nil {
		s.Fatal("Failed to send Tab: ", err)
	}

	if err := timeline.Start(ctx); err != nil {
		s.Fatal("Failed to start metrics: ", err)
	}
	if err := timeline.StartRecording(ctx); err != nil {
		s.Fatal("Failed to start recording: ", err)
	}
	const timelineDuration = time.Minute
	if err := testing.Sleep(ctx, timelineDuration); err != nil {
		s.Fatalf("Failed to wait %v: %v", timelineDuration, err)
	}
	pv, err := timeline.StopRecording()
	if err != nil {
		s.Fatal("Error while recording metrics: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save perf data: ", err)
	}
}
