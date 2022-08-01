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
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

type testType int

const (
	testTypeARC testType = iota
	testTypeBrowser
)

const (
	idleDuration   = 30 * time.Second
	emptyWindowURL = "about:blank"
)

type idlePerfTest struct {
	tracing     bool
	testType    testType
	browserType browser.Type
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
			Fixture:           "arcBootedRestricted",
		}, {
			Name:              "trace",
			ExtraSoftwareDeps: []string{"android_p"},
			Val: idlePerfTest{
				testType: testTypeARC,
				tracing:  true,
			},
			Fixture: "arcBootedRestricted",
		}, {
			Name:              "arcvm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val:               idlePerfTest{testType: testTypeARC},
			Fixture:           "arcBootedRestricted",
		}, {
			Name:              "arcvm_trace",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: idlePerfTest{
				testType: testTypeARC,
				tracing:  true,
			},
			Fixture: "arcBootedRestricted",
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val: idlePerfTest{
				testType:    testTypeBrowser,
				browserType: browser.TypeLacros,
			},
			Fixture: "lacrosPerf",
		}, {
			Name:              "lacros_trace",
			ExtraSoftwareDeps: []string{"lacros"},
			Val: idlePerfTest{
				testType:    testTypeBrowser,
				browserType: browser.TypeLacros,
				tracing:     true,
			},
			Fixture: "lacrosPerf",
		}, {
			Name: "ash",
			Val: idlePerfTest{
				testType:    testTypeBrowser,
				browserType: browser.TypeAsh,
			},
			Fixture: "chromeLoggedInDisableFirmwareUpdaterApp",
		}, {
			Name: "ash_trace",
			Val: idlePerfTest{
				testType:    testTypeBrowser,
				browserType: browser.TypeAsh,
				tracing:     true,
			},
			Fixture: "chromeLoggedInDisableFirmwareUpdaterApp",
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
	switch idleTest.testType {
	case testTypeARC:
		cr = s.FixtValue().(*arc.PreData).Chrome
		a = s.FixtValue().(*arc.PreData).ARC
	case testTypeBrowser:
		cr = s.FixtValue().(chrome.HasChrome).Chrome()
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
		if idleTest.testType == testTypeBrowser {
			// Sleep to get the baseline idle state without any browser open.
			if err := testing.Sleep(ctx, 10*time.Second); err != nil {
				return errors.Wrap(err, "failed to sleep")
			}

			conn, _, closeBrowser, err := browserfixt.SetUpWithURL(ctx, cr, idleTest.browserType, emptyWindowURL)
			if err != nil {
				s.Fatalf("Failed to open %s: %v", emptyWindowURL, err)
			}
			defer closeBrowser(closeCtx)
			defer conn.Close()
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
