// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ArcYoutubeCUJ,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Measures the performance of critical user journey for the YouTube ARC app",
		Contacts:     []string{"amusbach@chromium.org", "chromeos-perfmetrics-eng@google.com"},
		Attr:         []string{"group:cuj"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{cujrecorder.SystemTraceConfigFile},
		Fixture:      "loggedInToCUJUser",
		Timeout:      14 * time.Minute,
		Vars:         []string{"record"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func ArcYoutubeCUJ(ctx context.Context, s *testing.State) {
	const testDuration = 10 * time.Minute

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
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

	recorder, err := cujrecorder.NewRecorder(ctx, cr, tconn, a, cujrecorder.RecorderOptions{})
	if err != nil {
		s.Fatal("Failed to create the recorder: ", err)
	}
	defer recorder.Close(cleanupCtx)

	if err := recorder.AddCommonMetrics(tconn, tconn); err != nil {
		s.Fatal("Failed to add common metrics to recorder: ", err)
	}

	recorder.EnableTracing(s.OutDir(), s.DataPath(cujrecorder.SystemTraceConfigFile))

	if _, ok := s.Var("record"); ok {
		if err := recorder.AddScreenRecorder(ctx, tconn, s.TestName()); err != nil {
			s.Fatal("Failed to add screen recorder: ", err)
		}
	}

	// At the end of recorder.Run, the ARC app closes, meaning the
	// end screenshot would not show the state of the  Youtube app
	// itself. Take 1 screenshot at the middle of the test
	// and one at the end to ensure we see the state of the app.
	if err := recorder.AddScreenshotRecorder(ctx, testDuration/2, 2); err != nil {
		s.Fatal("Failed to add screenshot recorder: ", err)
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
		if err := testing.Sleep(ctx, testDuration); err != nil {
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
