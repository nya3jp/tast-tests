// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package benchmark

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/benchmark/setup"
	"chromiumos/tast/local/chrome/ash"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
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
			hwdep.Model("barla", "bluebird", "eve", "krane", "liara", "maple", "pyke", "kohaku"),
		),
		Timeout: 45 * time.Minute,
		Fixture: setup.BenchmarkARCFixture,
	})
}

func PCMarkWorkAndroidApp(ctx context.Context, s *testing.State) {
	const (
		pkgName     = "com.futuremark.pcmark.android.benchmark"
		appName     = "PCMark"
		resultLabel = "Work 2.0 performance score "
	)

	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kw.Close()

	uiDevice, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to setup ARC and Play Store: ", err)
	}
	defer uiDevice.Close(ctx)

	s.Log("Installing app from play store")
	if err := playstore.InstallApp(ctx, a, uiDevice, pkgName, -1); err != nil {
		s.Fatalf("Failed to install %s: %v", pkgName, err)
	}

	s.Log("Closing play store")
	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to close Play Store: ", err)
	}

	s.Log("Launch PCMark app")
	if err := kw.Accel(ctx, "search"); err != nil {
		s.Fatal("Failed to open search bar, error: ", err)
	}
	if err := kw.Type(ctx, appName); err != nil {
		s.Fatal("Failed to type, error: ", err)
	}
	param := chromeui.FindParams{ClassName: "SearchResultTileItemView"}
	node, err := chromeui.FindWithTimeout(ctx, tconn, param, 5*time.Second)
	if err != nil {
		s.Fatalf("Failed to search app: %s, error: %v", appName, err)
	}
	defer node.Release(ctx)
	uiPollOpt := testing.PollOptions{Timeout: 5 * time.Second, Interval: 1 * time.Second}
	if err := node.StableLeftClick(ctx, &uiPollOpt); err != nil {
		s.Fatal("Failed to click PCMark APP to launch it, error: ", err)
	}
	// Waiting for app to be visible.
	launchPollOpt := testing.PollOptions{Timeout: 15 * time.Second, Interval: 1 * time.Second}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return ash.WaitForVisible(ctx, tconn, pkgName)
	}, &launchPollOpt); err != nil {
		s.Fatalf("Failed to wait for the new window of %s, error: %v", pkgName, err)
	}

	// PCMark needs in-app installation before it can run. Check which button is available.
	btnRun := uiDevice.Object(ui.Text("RUN"), ui.Index(1))
	btnInstall := uiDevice.Object(ui.TextMatches(`INSTALL\(.*\)`)) // Match install button like "INSTALL(182MB)"
	btnInstallConfirm := uiDevice.Object(ui.Text("INSTALL"))
	var runBtnAvailable, installBtnAvaiable bool
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		runBtnAvailable = findUIObj(ctx, btnRun)
		if runBtnAvailable {
			return nil
		}
		installBtnAvaiable = findUIObj(ctx, btnInstall)
		if installBtnAvaiable {
			return nil
		}
		return errors.New("Neither RUN nor INSTALL button is available")
	}, &launchPollOpt); err != nil {
		s.Fatal("Failed to continue in PCMark APP: ", err)
	}
	if !runBtnAvailable && installBtnAvaiable {
		s.Log("Installing PCMark component")
		if err := clickUIObj(ctx, btnInstall); err != nil {
			s.Fatal("Failed to intsall PCMark component: ", err)
		}
		if err := clickUIObj(ctx, btnInstallConfirm); err != nil {
			s.Fatal("Failed to confirm PCMark component installation: ", err)
		}

		installPollOpt := testing.PollOptions{Timeout: 5 * time.Minute, Interval: 1 * time.Second}
		if err := testing.Poll(ctx, func(context.Context) error {
			// RUN button will show after the installation has finished.
			if found := findUIObj(ctx, btnRun); !found {
				return errors.Wrap(err, "component installation has not completed yet")
			}
			return nil
		}, &installPollOpt); err != nil {
			s.Fatal("Failed to wait for component installation: ", err)
		}
	}

	s.Log("Executing benchmark")
	if err := clickUIObj(ctx, btnRun); err != nil {
		s.Fatal("Failed to click RUN button to start benchmark test: ", err)
	}
	resultObj := uiDevice.Object(ui.TextContains(resultLabel))
	startTime := time.Now()
	execPollOpt := testing.PollOptions{Timeout: 30 * time.Minute, Interval: 10 * time.Second}
	if err := testing.Poll(ctx, func(context.Context) error {
		if err := resultObj.WaitForExists(ctx, 20*time.Millisecond); err != nil {
			endTime := time.Now()
			s.Logf("Result label not found - PCMark test is still running. Elapsed time: %s", endTime.Sub(startTime))
			return errors.Wrap(err, "result label not found")
		}
		return nil
	}, &execPollOpt); err != nil {
		s.Fatal("Failed to wait for benchmark to finish its execution: ", err)
	}

	resultText, err := resultObj.GetText(ctx)
	if err != nil {
		s.Fatal("Failed to get result text: ", err)
	}
	// Get the score from the test result text.
	strScore := strings.TrimSpace(resultText[len(resultLabel):])
	s.Logf("benchmark score: [%s]", strScore)

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
