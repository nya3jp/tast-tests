// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/inputlatency"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         KeyboardPerf,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Test ARC keyboard system performance",
		Contacts:     []string{"arc-performance@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Data:         inputlatency.AndroidData(),
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBooted",
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"android_p", "lacros"},
			Fixture:           "lacrosWithArcBooted",
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBooted",
		}, {
			Name:              "vm_lacros",
			ExtraSoftwareDeps: []string{"android_vm", "lacros"},
			Fixture:           "lacrosWithArcBooted",
		}},
		Timeout: 2 * time.Minute,
	})
}

func KeyboardPerf(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Could not open Test API connection: ", err)
	}

	a := s.FixtValue().(*arc.PreData).ARC
	d := s.FixtValue().(*arc.PreData).UIDevice

	s.Log("Creating virtual keyboard")
	kbd, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Unable to create virtual keyboard: ", err)
	}
	defer kbd.Close()

	if err := inputlatency.InstallArcHostClockClient(ctx, a, s); err != nil {
		s.Fatal("Could not install arc-host-clock-client: ", err)
	}

	const (
		apkName      = "ArcInputLatencyTest.apk"
		appName      = "org.chromium.arc.testapp.inputlatency"
		activityName = ".MainActivity"
	)
	s.Log("Installing " + apkName)
	if err := a.Install(ctx, arc.APKPath(apkName)); err != nil {
		s.Fatal("Failed to install the APK: ", err)
	}

	s.Logf("Launching %s/%s", appName, activityName)
	act, err := arc.NewActivity(a, appName, activityName)
	if err != nil {
		s.Fatalf("Unable to create new activity %s/%s: %v", appName, activityName, err)
	}
	defer act.Close()

	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		s.Fatalf("Unable to launch %s/%s: %v", appName, activityName, err)
	}
	defer act.Stop(ctx, tconn)

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait until CPU idle: ", err)
	}

	s.Log("Injecting key events")
	const (
		numEvents = 50
		waitMS    = 50
	)
	eventTimes := make([]int64, 0, numEvents)
	for i := 0; i < numEvents; i += 2 {
		if err := inputlatency.WaitForNextEventTime(ctx, a, &eventTimes, waitMS); err != nil {
			s.Fatal("Failed to generate event time: ", err)
		}
		if err := kbd.AccelPress(ctx, "a"); err != nil {
			s.Fatal("Unable to inject key events: ", err)
		}

		if err := inputlatency.WaitForNextEventTime(ctx, a, &eventTimes, waitMS); err != nil {
			s.Fatal("Failed to generate event time: ", err)
		}
		if err := kbd.AccelRelease(ctx, "a"); err != nil {
			s.Fatal("Unable to inject key events: ", err)
		}
	}

	pv := perf.NewValues()
	if err := inputlatency.EvaluateLatency(ctx, s, d, numEvents, eventTimes, "avgKeyboardLatency", pv); err != nil {
		s.Fatal("Failed to evaluate: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}
