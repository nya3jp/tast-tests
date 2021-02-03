// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package benchmark

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/benchmark/setup"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     PCMarkWorkAndroidApp,
		Desc:     "Execute PCMark Android App to do benchmark for PCMark Work and acquire test score",
		Contacts: []string{"alfredyu@cienet.com", "xliu@cienet.com"},
		// Purposely leave the empty Attr here. Public benchmark tests are not included in crosbolt group for now.
		Attr:         []string{},
		SoftwareDeps: []string{"arc", "chrome"},
		HardwareDeps: hwdep.D(
			hwdep.InternalDisplay(),
			// Since the public benchmark will publish data online, run it only on certain approved models.
			setup.PublicBenchmarkAllowed(),
		),
		Timeout: 45 * time.Minute,
		Fixture: setup.BenchmarkARCFixture,
	})
}

// PCMarkWorkAndroidApp executes PCMark Android App to do benchmark for PCMark Work and acquire test score
func PCMarkWorkAndroidApp(ctx context.Context, s *testing.State) {
	const (
		pkgName      = "com.futuremark.pcmark.android.benchmark"
		appName      = "PCMark"
		activityName = "com.futuremark.gypsum.activity.SplashPageActivity"
		resultLabel  = "Work 3.0 performance score "
	)

	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	uiDevice, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to setup ARC and Play Store: ", err)
	}

	cleanupCtx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	defer func(ctx context.Context) {
		uiDevice.Close(ctx)
		a.DumpUIHierarchyOnError(ctx, s.OutDir(), s.HasError)
		faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)
		w, err := ash.GetARCAppWindowInfo(ctx, tconn, pkgName)
		if err != nil {
			return
		}
		w.CloseWindow(ctx, tconn)
	}(cleanupCtx)

	// Test Procedure:
	// 1. Install APP from Play Store.
	// 2. Launch PCMark App.
	// 3. Install PCMark internal compoments.
	// 4. Run PCMark benchmark test and wait for results.
	// 5. Collect PCMark test result and save it to results_chart.json file.

	// 1. Install APP from Play Store.
	s.Log("Installing app from play store")
	if err := playstore.InstallApp(ctx, a, uiDevice, pkgName, -1); err != nil {
		s.Fatalf("Failed to install %s: %v", pkgName, err)
	}

	s.Log("Closing play store")
	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to close Play Store: ", err)
	}

	// 2. Launch PCMark App.
	s.Log("Launching PCMark app")
	if err := launchPCMark(ctx, tconn, uiDevice, a, pkgName, activityName); err != nil {
		s.Fatal("Failed to launch PCMark: ", err)
	}

	s.Log("Wait for home page properly rendered")
	obj := uiDevice.Object(ui.TextStartsWith("Benchmark performance and battery life with tests based on everyday activities."))
	if err := obj.WaitForExists(ctx, 2*time.Minute); err != nil {
		s.Fatal("Failed to continue in PCMark APP: ", err)
	}

	// 3. Install PCMark internal compoments.
	btnRun, err := installPCMarkComponents(ctx, uiDevice)
	if err != nil {
		s.Fatal("Failed to install PCMark components and get ready to run: ", err)
	}

	// 4. Run PCMark benchmark test and wait for results.
	s.Log("Executing benchmark")
	if err := btnRun.Click(ctx); err != nil {
		s.Fatal("Failed to click RUN button to start benchmark test: ", err)
	}
	resultObj := uiDevice.Object(ui.TextContains(resultLabel))
	startTime := time.Now()
	lastLogTime := startTime
	if err := testing.Poll(ctx, func(context.Context) error {
		if err := resultObj.WaitForExists(ctx, 20*time.Millisecond); err != nil {
			currentTime := time.Now()
			if currentTime.Sub(lastLogTime) > 30*time.Second {
				// Print log every 30 seconds.
				lastLogTime = currentTime
				s.Logf("Result label not found - PCMark test is still running. Elapsed time: %s", currentTime.Sub(startTime))
			}
			return errors.Wrap(err, "result label not found")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Minute, Interval: time.Second}); err != nil {
		s.Fatal("Failed to wait for benchmark to finish its execution: ", err)
	}

	// 5. Collect PCMark test result and save it to results_chart.json file.
	resultText, err := resultObj.GetText(ctx)
	if err != nil {
		s.Fatal("Failed to get result text: ", err)
	}
	// Get the score from the test result text.
	strScore := strings.TrimSpace(resultText[len(resultLabel):])
	s.Logf("Benchmark score: [%s]", strScore)
	// Take result screenshot which can be checked manually if needed.
	if err := screenshot.CaptureChrome(ctx, cr, filepath.Join(s.OutDir(), "result.png")); err != nil {
		s.Error("Failed to take screenshot: ", err)
	}

	fScore, err := strconv.ParseFloat(strScore, 64)
	if err != nil {
		s.Fatalf("Failed to parser score string %q: %v", strScore, err)
	}

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "Benchmark.PCMark",
		Unit:      "score",
		Direction: perf.BiggerIsBetter,
	}, float64(fScore))

	if err = pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store values: ", err)
	}
}

func launchPCMark(ctx context.Context, tconn *chrome.TestConn, device *ui.Device, ar *arc.ARC, pkg, activity string) error {
	act, err := arc.NewActivity(ar, pkg, activity)
	if err != nil {
		return errors.Wrap(err, "failed to create new activity")
	}
	defer act.Close()

	if err = act.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start app")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return ash.WaitForVisible(ctx, tconn, pkg)
	}, &testing.PollOptions{Timeout: 5 * time.Minute}); err != nil {
		return errors.Wrap(err, "failed to wait for the PCMark APP window")
	}

	// Click the "ACCEPT" button if it shows up.
	accept := device.Object(ui.TextContains("ACCEPT"))
	if found := findUIObj(ctx, accept); found {
		return clickUIObj(ctx, accept)
	}
	return nil
}

// installPCMarkComponents installs the PCMark's in-app software component,
// and returns the "RUN" button object.
func installPCMarkComponents(ctx context.Context, uiDevice *ui.Device) (*ui.Object, error) {
	// PCMark needs in-app installation before it can show the RUN button.
	// Check if RUN or "INSTALL" button is available.
	var btnRun, btnInstall *ui.Object
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if btnRun = findRunButton(ctx, uiDevice); btnRun != nil {
			return nil
		}
		if btnInstall = findInstallButton(ctx, uiDevice); btnInstall != nil {
			return nil
		}
		return errors.New("Neither RUN nor INSTALL button is available")
	}, &testing.PollOptions{Timeout: 1 * time.Minute, Interval: 1 * time.Second}); err != nil {
		return nil, err
	}
	if btnRun == nil && btnInstall != nil {
		testing.ContextLog(ctx, "Installing PCMark component")
		if err := btnInstall.Click(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to intsall PCMark component")
		}
		btnConfirm := findInstallConfirmButton(ctx, uiDevice)
		if btnConfirm == nil {
			return nil, errors.New("failed to find install confirm button")
		}
		if err := btnConfirm.Click(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to confirm PCMark component installation")
		}

		if err := testing.Poll(ctx, func(context.Context) error {
			// RUN button will shown after the installation has finished.
			if btnRun = findRunButton(ctx, uiDevice); btnRun == nil {
				return errors.New("component installation has not completed yet")
			}
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Minute, Interval: 1 * time.Second}); err != nil {
			return nil, errors.Wrap(err, "failed to wait for component installation to complete")
		}
	}
	return btnRun, nil
}

func findUIObj(ctx context.Context, obj *ui.Object) bool {
	if err := obj.WaitForExists(ctx, 10*time.Second); err != nil {
		return false
	}
	return true
}

func clickUIObj(ctx context.Context, obj *ui.Object) error {
	if found := findUIObj(ctx, obj); !found {
		return errors.New("failed to find ui object")
	}
	if err := obj.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click ui object")
	}
	return nil
}

// findRunButton finds the RUN button of PCMark.
// can change accross different runs - the text string might be in
// "field: text" or "field: content-description".
// Here use multiple ui.SelectorOption to find a ui object
func findRunButton(ctx context.Context, d *ui.Device) (obj *ui.Object) {
	text := "RUN"
	obj = d.Object(ui.Text(text))
	if findUIObj(ctx, obj) {
		return obj
	}
	obj = d.Object(ui.DescriptionContains(text))
	if findUIObj(ctx, obj) {
		return obj
	}
	return nil
}

// findInstallButton finds the install button of PCMark.
// PCMark's ui can change accross different runs - the text string might be in
// "field: text" or "field: content-description".
// Here use multiple ui.SelectorOption to find a ui object.
func findInstallButton(ctx context.Context, d *ui.Device) (obj *ui.Object) {
	// Match install button like "INSTALL(182MB)"
	regex := `INSTALL\(.*\)`
	obj = d.Object(ui.TextMatches(regex))
	if findUIObj(ctx, obj) {
		return obj
	}
	obj = d.Object(ui.DescriptionMatches(regex))
	if findUIObj(ctx, obj) {
		return obj
	}
	return nil
}

// findInstallConfirmButton finds the install confirm button of PCMark.
// PCMark's ui can change accross different runs the text string might be in
// "field: text" or "field: content-description".
// Here use multiple ui.SelectorOption to find a ui object.
func findInstallConfirmButton(ctx context.Context, d *ui.Device) (obj *ui.Object) {
	text := "INSTALL"
	obj = d.Object(ui.Text(text))
	if findUIObj(ctx, obj) {
		return obj
	}
	obj = d.Object(ui.DescriptionContains(text))
	if findUIObj(ctx, obj) {
		return obj
	}
	return nil
}
