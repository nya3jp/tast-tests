// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ArcYoutubeCUJ,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Measures the performance of critical user journey for the YouTube ARC app",
		Contacts:     []string{"amusbach@chromium.org", "chromeos-perfmetrics-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild", "group:cuj"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "loggedInToCUJUser",
		Timeout:      14 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func ArcYoutubeCUJ(ctx context.Context, s *testing.State) {
	test := cuj.Setup(ctx, s, nil)
	recorder := test.SetupRecorder(s)
	cleanupCtx, tconn := test.CloseCtx, test.Tconn
	defer test.Cleanup()

	a := s.FixtValue().(cuj.FixtureData).ARC

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(cleanupCtx)

	const ytAppPkgName = "com.google.android.youtube"
	if err := playstore.InstallApp(ctx, a, d, ytAppPkgName, &playstore.Options{}); err != nil {
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

	if err := recorder.AddCollectedMetrics(tconn, cujrecorder.DeprecatedMetricConfigs()...); err != nil {
		s.Fatal("Failed to add recorded metrics: ", err)
	}

	if err := recorder.Run(ctx, func(ctx context.Context) error {
		// Launch the ARC YouTube app.
		if err := act.Start(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to start ARC++ YouTube app")
		}
		defer act.Stop(cleanupCtx, tconn)

		// Go to https://www.youtube.com/watch?v=862r3XS2YB0
		if err := a.SendIntentCommand(ctx, "android.intent.action.VIEW", "vnd.youtube:862r3XS2YB0").Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to open https://www.youtube.com/watch?v=862r3XS2YB0")
		}

		// Wait for the ARC YouTube app to idle, so that we know the video has started actually playing.
		if err := d.WaitForIdle(ctx, 10*time.Second); err != nil {
			return errors.Wrap(err, "failed to wait for ARC++ YouTube app to idle")
		}

		// Sleep to simulate a user passively watching.
		if err := testing.Sleep(ctx, 1*time.Minute); err != nil {
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
