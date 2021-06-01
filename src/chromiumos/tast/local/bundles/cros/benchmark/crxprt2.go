// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package benchmark

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
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
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const crxprtRunningTime = 45 * time.Minute

func init() {
	testing.AddTest(&testing.Test{
		Func:     CRXPRT2,
		Desc:     "Execute Chrome extension CrXPRT 2 to do benchmark and acquire test score",
		Contacts: []string{"alfredyu@cienet.com", "xliu@cienet.com"},
		// Purposely leave the empty Attr here. Public benchmark tests are not included in crosbolt group for now.
		Attr:         []string{},
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      setup.BenchmarkChromeFixture,
		Timeout:      crxprtRunningTime + 15*time.Minute,
		VarDeps:      []string{"benchmark.username"},
	})
}

func CRXPRT2(ctx context.Context, s *testing.State) {
	const (
		extName     = "CrXPRT 2"
		windowName  = "CrXPRT"
		extID       = "ldeofhcgjhplegompgciolncekblpkad"
		extStoreURL = "https://chrome.google.com/webstore/detail/crxprt-2/ldeofhcgjhplegompgciolncekblpkad"
		extPageURL  = "chrome-extension://ldeofhcgjhplegompgciolncekblpkad/index.html"

		btnIDNext  = "next_btn"
		btnIDStart = "start_perf_btn"
	)

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

	// Dump UITree and save screenshot to provide information for debugging if something goes wrong.
	defer faillog.DumpUITreeOnError(closeCtx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(closeCtx, cr, s.OutDir(), s.HasError)

	ui := uiauto.New(tconn).WithPollOpts(testing.PollOptions{Timeout: 15 * time.Second, Interval: time.Second})

	root := nodewith.Name(windowName).Role(role.RootWebArea)
	footer := nodewith.ClassName("footer").Role(role.GenericContainer)
	if err := uiauto.Combine("wait for launch",
		ui.WaitUntilExists(root),
		ui.WaitUntilExists(footer),
	)(ctx); err != nil {
		s.Fatalf("Failed to wait for %s to launch: %v", extName, err)
	}

	crxprtNodeFinder := nodewith.Ancestor(root)

	// CrXPRT 2 provide two types of test, this test case will execute the performance test.
	s.Log("Select performance test")
	pTestFinder := crxprtNodeFinder.Name("Performance test").ClassName("test_sel_perfbtn").Role(role.Link).State("focusable", true).State("linked", true)
	if err := ui.LeftClick(pTestFinder)(ctx); err != nil {
		s.Fatal("Failed to select performance test: ", err)
	}

	s.Log("Connecting to the app")
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(extPageURL))
	if err != nil {
		s.Fatalf("Failed to connect to extension: %s, error: %+v", extName, err)
	}
	defer conn.Close()
	defer conn.CloseTarget(closeCtx)

	nextBtnFinder := crxprtNodeFinder.Name("Next").ClassName("blue_btn").Role(role.Link).State("focusable", true).State("linked", true)
	if err := ui.WaitUntilExists(nextBtnFinder)(ctx); err != nil {
		s.Fatal("Failed to wait for next button: ", err)
	}

	// The button might be offscreen or covered by footer,
	// and uiauto.Context.MakeVisible() will not work if the node is not offscreen or
	// partially covered (overlay) by other node (in this case, the footer might cover the button).
	// To make the click action more reliable, issue a click by invoking JavaScript call.
	clickByJS := fmt.Sprintf("document.getElementById(%q).click()", btnIDNext)
	if err := conn.Eval(ctx, clickByJS, nil); err != nil {
		s.Fatal("Failed to click button: ", err)
	}

	startBtn := crxprtNodeFinder.Name("Start").Role(role.Link)
	if err := ui.WaitForLocation(startBtn)(ctx); err != nil {
		s.Fatal("Failed to wait for start button to be stable: ", err)
	}
	info, err := ui.Info(ctx, startBtn)
	if err != nil {
		s.Fatal("Failed to get info of start button: ", err)
	}

	// CrXPRT 2 demand the device name to be set before execution.
	if info.ClassName == "gray_btn" {
		s.Log("Start button is grayed out; need to set up the deivce name")
		kb, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Failed to create the keyboard: ", err)
		}
		defer kb.Close()

		// Set the device name.
		s.Log("Typing device name")
		inputFieldFinder := crxprtNodeFinder.Role(role.TextField).State(state.Editable, true).State(state.Focusable, true)
		deviceName := s.RequiredVar("benchmark.username")
		if err := uiauto.Combine("input device name and wait for start button to change state",
			ui.LeftClickUntil(inputFieldFinder, ui.WithTimeout(3*time.Second).WaitUntilExists(inputFieldFinder.Focused())),
			kb.TypeAction(deviceName),
			ui.WaitUntilGone(startBtn.ClassName("gray_btn")),
		)(ctx); err != nil {
			s.Fatal("Failed to input device name and wait for start button to change state: ", err)
		}
	}

	// Make sure start button is enabled.
	if err := ui.WaitUntilExists(startBtn.ClassName("blue_btn"))(ctx); err != nil {
		s.Fatal("Failed to continue because start button is not enabled: ", err)
	}

	// The button might be offscreen or covered by footer,
	// and uiauto.Context.MakeVisible() will not work if the element is not offscreen or
	// partially covered (overlay) by other node (in this case, the footer might cover the button).
	// To make the click action more reliable, issue a click by invoke JavaScript call.
	clickByJS = fmt.Sprintf("document.getElementById(%q).click()", btnIDStart)
	if err := conn.Eval(ctx, clickByJS, nil); err != nil {
		s.Fatal("Failed to click button: ", err)
	}

	resultFinder := crxprtNodeFinder.Role(role.Heading).ClassName("selectable")
	incompleteFinder := resultFinder.Name("Test Incomplete")
	if err := ui.WaitUntilExists(incompleteFinder)(ctx); err != nil {
		s.Fatal("Benchmark might not have started executing: ", err)
	}

	startTime := time.Now()
	s.Logf("Benchmark %s is executing", extName)

	fScore := math.NaN()
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := ui.Gone(incompleteFinder)(ctx); err != nil {
			s.Logf("Result label not found - %s test is still running. Elapsed time: %s", extName, time.Since(startTime))
			return errors.Wrap(err, "still executing")
		}

		// There should be three nodes found, and second one is the target.
		resultNodeFinder := resultFinder.Nth(1)
		info, err := ui.Info(ctx, resultNodeFinder)
		if err != nil {
			return testing.PollBreak(errors.New("failed to get the info of result node"))
		}

		strScore := strings.TrimSpace(info.Name)
		if fScore, err = strconv.ParseFloat(strScore, 64); err != nil {
			return testing.PollBreak(errors.Wrapf(err, "failed to parser the result [%s] of benchmark", strScore))
		}

		return nil
	}, &testing.PollOptions{Timeout: crxprtRunningTime, Interval: 30 * time.Second}); err != nil {
		s.Fatal("Failed to get benchmark score, error: ", err)
	}

	if err := screenshot.CaptureChrome(ctx, cr, filepath.Join(s.OutDir(), "result.png")); err != nil {
		s.Error("Failed to take screenshot: ", err)
	}

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "Benchmark.CrXPRT",
		Unit:      "score",
		Direction: perf.BiggerIsBetter,
	}, fScore)
	s.Logf("Benchmark %s final score: %f", extName, fScore)
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
		testing.ContextLogf(ctx, "Chrome extension %s not exist, try to install", name)
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
		return errors.Wrapf(err, "failed to wait for %s to appear", name)
	}

	return nil
}
