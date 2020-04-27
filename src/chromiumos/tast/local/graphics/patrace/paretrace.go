// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package patrace provides a function to replay a PATrace (GLES)
// (https://github.com/ARM-software/patrace) in android
package patrace

import (
	"context"
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// RunTrace replays a PATrace (GLES) (https://github.com/ARM-software/patrace)
// in Android. APK and trace data are specified by apkFile and traceFile.
func RunTrace(ctx context.Context, s *testing.State, apkFile, traceFile string) {
	const (
		pkgName                = "com.arm.pa.paretrace"
		activityName           = ".Activities.RetraceActivity"
		tPowerSnapshotInterval = 5 * time.Second
	)

	// Shorten the test context so that even if the test times out
	// there will be time to clean up.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	// Reuse existing ARC and Chrome session.
	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome

	// Create Test API connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// setup.Setup configures a DUT for a test, and cleans up after.
	sup, cleanup := setup.New("paretrace")
	defer func() {
		if err := cleanup(cleanupCtx); err != nil {
			s.Fatal("Cleanup failed: ", err)
		}
	}()

	// Add the default power test configuration.
	sup.Add(setup.PowerTest(ctx, tconn))
	if err := sup.Check(ctx); err != nil {
		s.Fatal("Setup failed: ", err)
	}

	tabletCleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure tablet mode: ", err)
	}
	defer tabletCleanup(ctx)

	s.Log("Pushing trace file")

	out, err := a.Command(ctx, "mktemp", "-d", "-p", "/sdcard/Download").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	tmpDir := strings.TrimSpace(string(out))
	defer a.RemoveAll(ctx, tmpDir)

	s.Log("Temp dir: ", tmpDir)

	tracePath := filepath.Join(tmpDir, traceFile)
	resultPath := filepath.Join(tmpDir, traceFile+".result.json")

	if err := a.PushFile(ctx, s.DataPath(traceFile), tracePath); err != nil {
		s.Fatal("Failed to push the trace file: ", err)
	}

	if err := a.Install(ctx, s.DataPath(apkFile), arc.InstallOptionGrantPermissions); err != nil {
		s.Fatalf("Failed to install %s: %v", s.DataPath(apkFile), err)
	}

	act, err := arc.NewActivity(a, pkgName, activityName)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	metrics, err := perf.NewTimeline(ctx, power.TestMetrics(), perf.Interval(tPowerSnapshotInterval))
	if err != nil {
		s.Fatal("Failed to build metrics: ", err)
	}

	s.Log("Starting activity")

	if err := act.StartWithArgs(ctx, tconn, []string{"-W", "-S", "-n"}, []string{"--es", "fileName", tracePath, "--es", "resultFile", resultPath}); err != nil {
		s.Fatal("Cannot start retrace: ", err)
	}

	sdkVer, err := arc.SDKVersion()
	if err != nil {
		s.Fatal("Failed to get SDK version: ", err)
	}
	if sdkVer >= arc.SDKQ {
		// "This app was built for an older version of Android and may not work properly"
		// This button confirms it.
		versionOkButton := d.Object(ui.Text("OK"), ui.PackageName("android"))
		if err := versionOkButton.WaitForExists(ctx, 5*time.Second); err != nil {
			s.Fatal("Failed to find \"This app was built for an older version of Android and may not work properly\" dialog: ", err)
		}
		versionOkButton.Click(ctx)
	}

	exp := regexp.MustCompile(`paretrace32\s*:.*=+\sStart\stimer.*=+`)
	if err := a.WaitForLogcat(ctx, arc.RegexpPred(exp)); err != nil {
		s.Fatal("WaitForLogcat failed: ", err)
	}

	if err := metrics.Start(ctx); err != nil {
		s.Fatal("Failed to start metrics: ", err)
	}

	if err := metrics.StartRecording(ctx); err != nil {
		s.Fatal("Failed to start recording: ", err)
	}

	exp = regexp.MustCompile(`paretrace32\s*:.*=+\sEnd\stimer.*=+`)
	if err := a.WaitForLogcat(ctx, arc.RegexpPred(exp)); err != nil {
		s.Fatal("WaitForLogcat failed: ", err)
	}

	perfValues, err := metrics.StopRecording()
	if err != nil {
		s.Fatal("Error while recording power metrics: ", err)
	}

	defer func() {
		if err := perfValues.Save(s.OutDir()); err != nil {
			s.Error("Cannot save perf data: ", err)
		}
	}()

	// Wait for app cleanup
	if err := act.WaitForFinished(ctx, ctxutil.MaxTimeout); err != nil {
		s.Fatal("waitForFinished failed: ", err)
	}

	if err := setPerf(ctx, a, perfValues, resultPath); err != nil {
		s.Fatal("Failed to set perf values: ", err)
	}
}

// setPerf reads the performance numbers from the result file of paretrace, and
// store the values in perfValues
func setPerf(ctx context.Context, a *arc.ARC, perfValues *perf.Values, resultPath string) error {
	buf, err := a.ReadFile(ctx, resultPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read result file %q; paretrace did not finish successfully", resultPath)
	}

	var m struct {
		Results []struct {
			Time float64 `json:"time"`
			FPS  float64 `json:"fps"`
		} `json:"result"`
	}
	if err := json.Unmarshal(buf, &m); err != nil {
		return err
	}

	result := m.Results[0]

	perfValues.Set(
		perf.Metric{
			Name:      "duration",
			Unit:      "s",
			Direction: perf.SmallerIsBetter,
			Multiple:  false,
		}, result.Time)
	perfValues.Set(
		perf.Metric{
			Name:      "fps",
			Unit:      "fps",
			Direction: perf.BiggerIsBetter,
			Multiple:  false,
		}, result.FPS)

	testing.ContextLogf(ctx, "Duration: %fs, fps: %f", result.Time, result.FPS)

	return nil
}
