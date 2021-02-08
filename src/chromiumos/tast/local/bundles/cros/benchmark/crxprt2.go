// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package benchmark

import (
	"context"
	"math"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/benchmark/setup"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/cws"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     CRXPRT2,
		Desc:     "Execute Chrome extension CrXPRT 2 to do benchmark and acquire test score",
		Contacts: []string{"alfredyu@cienet.com", "xliu@cienet.com"},
		// Purposely leave the empty Attr here. Public benchmark tests are not included in crosbolt group for now.
		Attr:         []string{},
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(
			hwdep.InternalDisplay(),
			// Since the public benchmark will publish data online, run it only on certain approved models.
			setup.PublicBenchmarkAllowed(),
		),
		Fixture: setup.BenchmarkChromeFixture,
		Timeout: 1 * time.Hour,
		Vars:    []string{"benchmark.username"},
	})
}

func CRXPRT2(ctx context.Context, s *testing.State) {
	const (
		extName     = "CrXPRT 2"
		extID       = "ldeofhcgjhplegompgciolncekblpkad"
		extStoreURL = "https://chrome.google.com/webstore/detail/crxprt-2/ldeofhcgjhplegompgciolncekblpkad"
		extPageURL  = "chrome-extension://ldeofhcgjhplegompgciolncekblpkad/index.html"

		btnIDNext  = "next_btn"
		btnIDStart = "start_perf_btn"

		inputFieldDivID = "page_2_item1_div1"
		inputFieldID    = "page_2_item1_devicename"
	)
	execPollOpt := testing.PollOptions{Timeout: 45 * time.Minute, Interval: 30 * time.Second}

	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	s.Logf("Launching %s", extName)
	if err := launchChromeExtension(ctx, cr, tconn, extName, extID, extStoreURL); err != nil {
		s.Fatalf("Failed to launch %s, error: %v", extName, err)
	}

	// Take screen shot to provide information for debug if something goes wrong.
	defer faillog.DumpUITreeOnError(closeCtx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(closeCtx, cr, s.OutDir(), s.HasError)

	ui := uiauto.New(tconn).WithPollOpts(testing.PollOptions{Timeout: 15 * time.Second, Interval: time.Second})

	// CrXPRT 2 provide two types of test, this test case will execute the performance test.
	// Select performance test and click next button to continue.
	pTestFinder := nodewith.Name("Performance test").ClassName("test_sel_perfbtn").Role(role.Link).State("focusable", true).State("linked", true)
	nextBtnFinder := nodewith.Name("Next").ClassName("blue_btn").Role(role.Link).State("focusable", true).State("linked", true)
	if err := uiauto.Combine("select performance test and click next button",
		ui.WaitUntilExists(pTestFinder),
		ui.LeftClickUntil(nextBtnFinder, ui.Gone(nextBtnFinder)),
	)(ctx); err != nil {
		s.Fatal("Failed to select performance test and click next button: ", err)
	}

	startBtn := nodewith.Name("Start").Role(role.Link).First()
	info, err := ui.Info(ctx, startBtn)
	if err != nil {
		s.Fatal("Failed to get info of start button: ", err)
	}

	// CrXPRT 2 demand the device name be set before execution.
	if info.ClassName == "gray_btn" {
		kb, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Failed to create the keyboard: ", err)
		}
		defer kb.Close()

		// Set the device name.
		inputFieldFinder := nodewith.Role(role.TextField).State(state.Editable, true).State(state.Focusable, true)
		inputFieldFocused := nodewith.Role(role.TextField).Focused()
		deviceName := s.RequiredVar("benchmark.username")
		if err := uiauto.Combine("input device name",
			ui.LeftClickUntil(inputFieldFinder, ui.Exists(inputFieldFocused)),
			kb.TypeAction(deviceName),
		)(ctx); err != nil {
			s.Fatal("Failed to input device name: ", err)
		}
	}

	// Once the text is typed, info of start button should changed.
	info, err = ui.Info(ctx, startBtn)
	if err != nil {
		s.Fatal("Failed to get info of start button: ", err)
	}
	// Once the text is typed, start button should able to click.
	if info.ClassName == "gray_btn" {
		s.Fatal("Failed to continue, start button is gray out: ", err)
	}

	// Click button to start.
	if err := ui.LeftClickUntil(startBtn, ui.Gone(startBtn))(ctx); err != nil {
		s.Fatal("Failed to click button to start benchmark: ", err)
	}
	startTime := time.Now()
	s.Logf("Benchmark %s is executing", extName)

	resultFinder := nodewith.Role(role.Heading).ClassName("selectable")
	fScore := math.NaN()
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		infos, err := ui.NodesInfo(ctx, resultFinder)
		// There should be three nodes found.
		if err != nil || len(infos) != 3 {
			s.Logf("Result label not found - %s test is still running. Elapsed time: %s", extName, time.Since(startTime))
			return testing.PollBreak(errors.New("unexpected UI result"))
		}

		// Second one is the target.
		strScore := infos[1].Name
		strScore = strings.TrimSpace(strScore)

		if strScore == "Test Incomplete" {
			s.Logf("Result label not found - %s test is still running. Elapsed time: %s", extName, time.Since(startTime))
			return errors.Wrap(err, "still executing")
		}

		if fScore, err = strconv.ParseFloat(strScore, 64); err != nil {
			return testing.PollBreak(errors.Wrapf(err, "failed to parser the result [%s] of benchmark", strScore))
		}

		return nil
	}, &execPollOpt); err != nil {
		s.Fatal("Failed get benchmark score, error: ", err)
	}

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "Benchmark.CrXPRT",
		Unit:      "score",
		Direction: perf.BiggerIsBetter,
	}, fScore)
	if err = pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to store values, error: ", err)
	}
}

func launchChromeExtension(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, name, ID, URL string) error {
	isInstalled, err := ash.ChromeAppInstalled(ctx, tconn, ID)
	if err != nil {
		return errors.Wrap(err, "failed to check Chrome app existance")
	}
	if !isInstalled {
		app := cws.App{Name: name,
			URL:          URL,
			InstalledTxt: "Launch app",
			AddTxt:       "Add to Chrome",
			ConfirmTxt:   "Add app",
		}
		if err := cws.InstallApp(ctx, cr, tconn, app); err != nil {
			return errors.Wrapf(err, "failed to install %s", name)
		}
	}
	if err := apps.Launch(ctx, tconn, ID); err != nil {
		return errors.Wrapf(err, "failed to launch %s", name)
	}
	if err := ash.WaitForApp(ctx, tconn, ID, time.Minute); err != nil {
		return errors.Wrapf(err, "failed to wait %s appear", name)
	}

	return nil
}
