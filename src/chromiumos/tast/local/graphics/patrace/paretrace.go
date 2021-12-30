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

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
)

// RunTrace replays a PATrace (GLES) (https://github.com/ARM-software/patrace)
// in Android. APK and trace data are specified by apkFile and traceFile.
func RunTrace(ctx context.Context, preData arc.PreData, apkFile, traceFile, outDir string, offscreen bool) (retErr error) {
	const (
		pkgName                = "com.arm.pa.paretrace"
		activityName           = ".Activities.RetraceActivity"
		tPowerSnapshotInterval = 5 * time.Second
	)

	handleDeferError := func(err error) {
		if retErr != nil {
			testing.ContextLog(ctx, "Failed in RunTrace() defer: ", err)
		} else {
			retErr = err
		}
	}

	// Shorten the test context so that even if the test times out
	// there will be time to clean up.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	// Reuse existing ARC and Chrome session.
	a := preData.ARC
	cr := preData.Chrome

	// Create Test API connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Test API connection")
	}

	// setup.Setup configures a DUT for a test, and cleans up after.
	sup, cleanup := setup.New("paretrace")
	defer func() {
		if err := cleanup(cleanupCtx); err != nil {
			handleDeferError(errors.Wrap(err, "cleanup failed"))
		}
	}()

	// Add the default power test configuration.
	sup.Add(setup.PowerTest(ctx, tconn, setup.PowerTestOptions{
		Wifi: setup.DisableWifiInterfaces, Battery: setup.ForceBatteryDischarge, NightLight: setup.DisableNightLight}))
	if err := sup.Check(ctx); err != nil {
		return errors.Wrap(err, "setup failed")
	}

	tabletCleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		return errors.Wrap(err, "failed to ensure tablet mode")
	}
	defer tabletCleanup(ctx)

	testing.ContextLog(ctx, "Pushing trace file")

	out, err := a.Command(ctx, "mktemp", "-d", "-p", "/sdcard/Download").Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to create temp dir")
	}
	tmpDir := strings.TrimSpace(string(out))
	defer a.RemoveAll(ctx, tmpDir)

	testing.ContextLog(ctx, "Temp dir: ", tmpDir)

	traceName := filepath.Base(traceFile)
	tracePath := filepath.Join(tmpDir, traceName)
	resultPath := filepath.Join(tmpDir, traceName+".result.json")

	if err := a.PushFile(ctx, traceFile, tracePath); err != nil {
		return errors.Wrap(err, "failed to push the trace file")
	}

	if err := a.Install(ctx, apkFile, adb.InstallOptionGrantPermissions); err != nil {
		return errors.Wrapf(err, "failed to install %s", apkFile)
	}

	act, err := arc.NewActivity(a, pkgName, activityName)
	if err != nil {
		return errors.Wrap(err, "failed to create new activity")
	}
	defer act.Close()

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "failed initializing UI Automator")
	}
	defer d.Close(ctx)

	metrics, err := perf.NewTimeline(ctx, power.TestMetrics(), perf.Interval(tPowerSnapshotInterval))
	if err != nil {
		return errors.Wrap(err, "failed to build metrics")
	}

	testing.ContextLog(ctx, "Starting activity")

	opts := []arc.ActivityStartOption{
		arc.WithWaitForLaunch(),
		arc.WithForceStop(),
		arc.WithExtraString("fileName", tracePath),
		arc.WithExtraString("resultFile", resultPath),
		arc.WithExtraBool("force_single_window", true),
	}

	if offscreen {
		opts = append(opts, arc.WithExtraBool("forceOffscreen", true))
	}

	if err := act.Start(ctx, tconn, opts...); err != nil {
		return errors.Wrap(err, "cannot start retrace")
	}

	sdkVer, err := arc.SDKVersion()
	if err != nil {
		return errors.Wrap(err, "failed to get SDK version")
	}
	if sdkVer >= arc.SDKQ {
		// "This app was built for an older version of Android and may not work properly"
		// This button confirms it.
		versionOkButton := d.Object(ui.Text("OK"), ui.PackageName("android"))
		if err := versionOkButton.WaitForExists(ctx, 5*time.Second); err != nil {
			return errors.Wrap(err, "failed to find \"This app was built for an older version of Android and may not work properly\" dialog")
		}
		versionOkButton.Click(ctx)
	}

	crashOrOOM := false
	quitFunc := func() bool {
		isRunning, err := act.IsRunning(ctx)
		if err != nil {
			return false
		}
		if !isRunning {
			testing.ContextLog(ctx, "Activity is no longer running")
			crashOrOOM = true
			return true
		}
		return false
	}

	exp := regexp.MustCompile(`paretrace(32|64)\s*:.*=+\sStart\stimer.*=+`)
	if err := a.WaitForLogcat(ctx, arc.RegexpPred(exp), quitFunc); err != nil {
		return errors.Wrap(err, "failed to find paretrace \"Start timer\"")
	}
	if crashOrOOM {
		return errors.New("there was either a crash or an OOM")
	}

	if err := metrics.Start(ctx); err != nil {
		return errors.Wrap(err, "failed to start metrics")
	}

	if err := metrics.StartRecording(ctx); err != nil {
		return errors.Wrap(err, "failed to start recording")
	}

	exp = regexp.MustCompile(`paretrace(32|64)\s*:.*=+\sEnd\stimer.*=+`)
	if err := a.WaitForLogcat(ctx, arc.RegexpPred(exp), quitFunc); err != nil {
		return errors.Wrap(err, "failed to find paretrace \"End timer\"")
	}
	if crashOrOOM {
		return errors.New("there was either a crash or an OOM")
	}

	perfValues, err := metrics.StopRecording(ctx)
	if err != nil {
		return errors.Wrap(err, "error while recording power metrics")
	}

	defer func() {
		if err := perfValues.Save(outDir); err != nil {
			handleDeferError(errors.Wrap(err, "cannot save perf data"))
		}
	}()

	// Wait for app cleanup
	if err := act.WaitForFinished(ctx, ctxutil.MaxTimeout); err != nil {
		return errors.Wrap(err, "failed to wait for activity finishing")
	}

	if err := setPerf(ctx, a, perfValues, resultPath); err != nil {
		return errors.Wrap(err, "failed to set perf values")
	}

	return nil
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
