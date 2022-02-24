// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ArcYoutubeCUJ,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Measures the performance of critical user journey for the YouTube ARC app",
		Contacts:     []string{"amusbach@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "loggedInToCUJUser",
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func ArcYoutubeCUJ(ctx context.Context, s *testing.State) {
	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(cuj.FixtureData).Chrome
	a := s.FixtValue().(cuj.FixtureData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(cleanupCtx)

	const ytAppPkgName = "com.google.android.youtube"
	if err := playstore.InstallApp(ctx, a, d, ytAppPkgName, 3); err != nil {
		s.Fatal("Failed to install ARC++ YouTube app: ", err)
	}

	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to close Play Store: ", err)
	}

	act, err := arc.NewActivity(a, ytAppPkgName, "com.google.android.apps.youtube.app.WatchWhileActivity")
	if err != nil {
		s.Fatal("Failed to create ARC++ YouTube app activity: ", err)
	}
	defer act.Close()

	recorder, err := cuj.NewRecorder(ctx, cr, a, cuj.MetricConfigs()...)
	if err != nil {
		s.Fatal("Failed to create the recorder: ", err)
	}
	defer recorder.Close(cleanupCtx)

	if err := recorder.Run(ctx, func(ctx context.Context) error {
		if err := act.Start(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to start ARC++ YouTube app")
		}
		defer act.Stop(cleanupCtx, tconn)

		if err := d.WaitForIdle(ctx, time.Minute); err != nil {
			return errors.Wrap(err, "failed to wait for ARC++ YouTube app to idle")
		}

		if err := d.Object(
			ui.ClassName("android.widget.ImageView"),
			ui.Description("Search"),
			ui.PackageName("com.google.android.youtube"),
		).Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click Search")
		}

		if err := d.WaitForIdle(ctx, 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait for ARC++ YouTube app to idle")
		}

		if err := d.Object(
			ui.Text("Search YouTube"),
			ui.ClassName("android.widget.EditText"),
			ui.PackageName("com.google.android.youtube"),
		).SetText(ctx, "\"Top 100 Free Stock Videos 4K Rview and Download in Pixabay 12/2018\""); err != nil {
			return errors.Wrap(err, "failed to set search query")
		}

		if err := d.PressKeyCode(ctx, ui.KEYCODE_ENTER, 0); err != nil {
			return errors.Wrap(err, "failed to press Enter")
		}

		if err := d.WaitForIdle(ctx, 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait for ARC++ YouTube app to idle")
		}

		if err := d.Object(
			ui.ClassName("android.view.ViewGroup"),
			ui.DescriptionMatches("Top 100 Free Stock Videos 4K Rview and Download in Pixabay 12 2018 - 41 minutes - .+ - play video"),
			ui.PackageName("com.google.android.youtube"),
		).Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click for video")
		}

		if err := d.WaitForIdle(ctx, 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait for ARC++ YouTube app to idle")
		}

		if err := testing.Sleep(ctx, time.Minute); err != nil {
			return errors.Wrap(err, "failed to sleep")
		}

		return nil
	}); err != nil {
		s.Fatal("Failed to conduct the performance measurement: ", err)
	}

	pv := perf.NewValues()
	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to record the performance data: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save the performance data: ", err)
	}
}
