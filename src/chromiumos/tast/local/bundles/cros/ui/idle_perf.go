// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

type testType string

const (
	testTypeARC    testType = "arc"
	testTypeAsh    testType = "ash"
	testTypeLacros testType = "lacros"
)

const (
	idleDuration   = 30 * time.Second
	emptyWindowURL = "about:blank"
)

type idlePerfTest struct {
	tracing  bool
	testType testType
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         IdlePerf,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the CPU usage while the desktop is idle",
		Contacts:     []string{"xiyuan@chromium.org", "yichenz@chromium.org", "chromeos-perfmetrics-eng@google.com"},
		Attr:         []string{"group:cuj"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{cujrecorder.SystemTraceConfigFile},
		Timeout:      cuj.CPUStablizationTimeout + idleDuration,

		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Val:               idlePerfTest{testType: testTypeARC},
			Pre:               arc.Booted(),
		}, {
			Name:              "trace",
			ExtraSoftwareDeps: []string{"android_p"},
			Val:               idlePerfTest{testType: testTypeARC, tracing: true},
			Pre:               arc.Booted(),
		}, {
			Name:              "arcvm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val:               idlePerfTest{testType: testTypeARC},
			Pre:               arc.Booted(),
		}, {
			Name:              "arcvm_trace",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val:               idlePerfTest{testType: testTypeARC, tracing: true},
			Pre:               arc.Booted(),
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               idlePerfTest{testType: testTypeLacros},
			Fixture:           "loggedInToCUJUserLacros",
		}, {
			Name:              "lacros_trace",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               idlePerfTest{testType: testTypeLacros, tracing: true},
			Fixture:           "loggedInToCUJUserLacros",
		}, {
			Name:    "ash",
			Val:     idlePerfTest{testType: testTypeAsh},
			Fixture: "loggedInToCUJUser",
		}, {
			Name:    "ash_trace",
			Val:     idlePerfTest{testType: testTypeAsh, tracing: true},
			Fixture: "loggedInToCUJUser",
		}},
	})
}

func IdlePerf(ctx context.Context, s *testing.State) {
	idleTest := s.Param().(idlePerfTest)

	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	var cr *chrome.Chrome
	var a *arc.ARC
	var tconn *chrome.TestConn
	if idleTest.testType == testTypeARC {
		cr = s.PreValue().(arc.PreData).Chrome
		a = s.PreValue().(arc.PreData).ARC
	} else {
		cr = s.FixtValue().(chrome.HasChrome).Chrome()

		var err error
		tconn, err = cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to connect to the test API connection: ", err)
		}
	}

	// Wait for cpu to stabilize before test.
	if err := cpu.WaitUntilStabilized(ctx, cuj.CPUCoolDownConfig()); err != nil {
		// Log the cpu stabilizing wait failure instead of make it fatal.
		// TODO(b/213238698): Include the error as part of test data.
		s.Log("Failed to wait for CPU to become idle: ", err)
	}

	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	// Recorder with no additional config; it records and reports memory usage and
	// CPU percents of browser/GPU processes.
	recorder, err := cujrecorder.NewRecorder(ctx, cr, a, cujrecorder.RecorderOptions{})
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}
	defer func() {
		if err := recorder.Close(closeCtx); err != nil {
			s.Error("Failed to stop recorder: ", err)
		}
	}()
	if idleTest.tracing {
		recorder.EnableTracing(s.OutDir(), s.DataPath(cujrecorder.SystemTraceConfigFile))
	}

	if err := recorder.Run(ctx, func(ctx context.Context) error {
		if idleTest.testType != testTypeARC {
			// Sleep to get the baseline idle state without any browser open.
			if err := testing.Sleep(ctx, 10*time.Second); err != nil {
				return errors.Wrap(err, "failed to sleep")
			}

			// Open a New Tab window.
			switch idleTest.testType {
			case testTypeAsh:
				if _, err := cr.NewConn(ctx, emptyWindowURL); err != nil {
					return errors.Wrapf(err, "failed to open %s for Ash", emptyWindowURL)
				}
			case testTypeLacros:
				if _, err := lacros.LaunchWithURL(ctx, tconn, emptyWindowURL); err != nil {
					return errors.Wrapf(err, "failed to open %s for Lacros", emptyWindowURL)
				}
			}
		}

		s.Log("Just wait for ", idleDuration, " to check the load of idle status")
		return testing.Sleep(ctx, idleDuration)
	}); err != nil {
		s.Fatal("Failed to run the test scenario: ", err)
	}

	pv := perf.NewValues()
	if err = recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to report: ", err)
	}
	if err = pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store values: ", err)
	}
}
