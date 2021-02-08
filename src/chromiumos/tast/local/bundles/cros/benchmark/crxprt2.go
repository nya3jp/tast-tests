// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package benchmark

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/benchmark/setup"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
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
		Vars: []string{
			"benchmark.username",
		},
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
	defaultPollOpt := testing.PollOptions{Timeout: 1 * time.Minute, Interval: 2 * time.Second}
	execPollOpt := testing.PollOptions{Timeout: 45 * time.Minute, Interval: 30 * time.Second}

	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	launchCtx, cancel := context.WithTimeout(ctx, execPollOpt.Timeout)
	defer cancel()

	s.Logf("Launching %s", extName)
	if err := launchChromeExtension(ctx, cr, tconn, extName, extID, extStoreURL); err != nil {
		s.Fatalf("Failed to launch %s, error: %v", extName, err)
	}

	// Wait for couple of seconds for the page becomes available.
	if err := testing.Sleep(ctx, 2*time.Second); err != nil {
		s.Fatal("Failed to sleep to wait for extension page: ", err)
	}

	s.Log("Connecting to the app")
	conn, err := cr.NewConnForTarget(launchCtx, chrome.MatchTargetURL(extPageURL))
	if err != nil {
		s.Fatalf("Failed to connect to extension: %s, error: %+v", extName, err)
	}
	defer conn.Close()
	defer conn.CloseTarget(launchCtx)

	s.Log("Clicking performance section")
	if err := waitPageReady(ctx, conn, &defaultPollOpt); err != nil {
		s.Fatal("Failed to wait page ready, error: ", err)
	} else {
		className := "test_sel_perfbtn"
		code := fmt.Sprintf("document.getElementsByClassName(%q)[0].click()", className)
		if err := conn.Eval(ctx, code, nil); err != nil {
			s.Fatal("Failed to click selection, error: ", err)
		}
	}

	s.Log("Clicking button: 'next'")
	if err := clickButton(launchCtx, conn, btnIDNext, &defaultPollOpt); err != nil {
		s.Fatal("Failed to click button 'next', error: ", err)
	}

	if err := waitPageReady(ctx, conn, &defaultPollOpt); err != nil {
		s.Fatal("Failed to wait for page ready, error: ", err)
	}

	// Set the device name, CrXPRT 2 demand the device name be set before execution.
	if shown, err := isElementShown(ctx, conn, inputFieldDivID); err != nil {
		s.Fatal("Failed to check element appearance, error: ", err)
	} else if shown {
		s.Log("Setting device name")
		deviceName := s.RequiredVar("benchmark.username")
		code := fmt.Sprintf("document.getElementById(%q).value=%q", inputFieldID, deviceName)
		if err := conn.Eval(ctx, code, nil); err != nil {
			s.Fatal("Failed to input device name, error: ", err)
		}
		code = "checkDeviceName()"
		if err := conn.Eval(ctx, code, nil); err != nil {
			s.Fatal("Failed to apply change of device name, error: ", err)
		}
	}

	s.Log("Clicking button to start execute")
	if err := clickButton(launchCtx, conn, btnIDStart, &defaultPollOpt); err != nil {
		s.Fatal("Failed to click button to start execute, error: ", err)
	}

	var strScore string
	fScore := math.NaN()
	startTime := time.Now()
	s.Logf("Benchmark %s is executing", extName)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := waitPageReady(ctx, conn, &defaultPollOpt); err != nil {
			return errors.Wrap(err, "page is not ready")
		}
		code := fmt.Sprintf("document.getElementById(%q).innerHTML", "page_3_perfScore")
		if err := conn.Eval(ctx, code, &strScore); err != nil {
			return errors.Wrap(err, "failed to get HTML element content")
		}
		// benchmark complete until the element shows proper number
		strScore = strings.TrimSpace(strScore)
		if fScore, err = strconv.ParseFloat(strScore, 64); err != nil {
			endTime := time.Now()
			s.Logf("Result label not found - %s test is still running. Elapsed time: %s", extName, endTime.Sub(startTime))
			return errors.Wrap(err, "still executing")
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

func isElementShown(ctx context.Context, conn *chrome.Conn, eleID string) (bool, error) {
	code := fmt.Sprintf("document.getElementById(%q).style.display", eleID)

	var status string
	if err := conn.Eval(ctx, code, &status); err != nil {
		return false, errors.Wrap(err, "failed to check element's display status")
	}
	if status != "none" {
		return true, nil
	}
	return false, nil
}

func waitElementShown(ctx context.Context, conn *chrome.Conn, eleID string, pollOpt *testing.PollOptions) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if shown, err := isElementShown(ctx, conn, eleID); err != nil || !shown {
			return errors.Wrap(err, "element not shown yet")
		}
		return nil
	}, pollOpt); err != nil {
		return errors.Wrap(err, "failed to wait for element appear")
	}

	return nil
}

func waitPageReady(ctx context.Context, conn *chrome.Conn, pollOpt *testing.PollOptions) error {
	code := `document.readyState==="complete"`

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ready := false
		if err := conn.Eval(ctx, code, &ready); err != nil {
			return errors.Wrapf(err, "failed to execute script: %s", code)
		} else if !ready {
			return errors.Wrap(err, "page is not ready yet")
		}
		return nil
	}, pollOpt); err != nil {
		return errors.New("failed to wait until page ready")
	}

	return nil
}

func clickButton(ctx context.Context, conn *chrome.Conn, btnID string, pollOpt *testing.PollOptions) error {
	if err := waitPageReady(ctx, conn, pollOpt); err != nil {
		return err
	}
	if err := waitElementShown(ctx, conn, btnID, pollOpt); err != nil {
		return err
	}
	code := fmt.Sprintf("document.getElementById(%q).click()", btnID)
	if err := conn.Eval(ctx, code, nil); err != nil {
		return errors.Wrap(err, "failed to click button")
	}
	return nil
}

func launchChromeExtension(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, name, ID, URL string) error {
	isInstalled, err := ash.ChromeAppInstalled(ctx, tconn, ID)
	if err != nil {
		return errors.Wrap(err, "failed to check Chrome app existance")
	}
	if !isInstalled {
		if err := installChromeExt(ctx, cr, tconn, URL); err != nil {
			return errors.Wrap(err, "failed to install CrXPRT 2")
		}
	}
	if err := apps.Launch(ctx, tconn, ID); err != nil {
		return errors.Wrap(err, "failed to launch CrXPRT 2")
	}
	if err := ash.WaitForApp(ctx, tconn, ID); err != nil {
		return errors.Wrapf(err, "failed to wait %s appear", ID)
	}

	return nil
}

// installChromeExt installs the Chrome app by given Chrome extension URL.
func installChromeExt(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, url string) error {
	cws, err := cr.NewConn(ctx, url)
	if err != nil {
		return err
	}
	defer cws.Close()
	defer cws.CloseTarget(ctx)

	var (
		addClicked       = false
		paramsLaunch     = ui.FindParams{Name: "Launch app", Role: ui.RoleTypeButton}
		paramsAdd2Chrome = ui.FindParams{Name: "Add to Chrome", Role: ui.RoleTypeButton}
		paramsAddApp     = ui.FindParams{Name: "Add app", Role: ui.RoleTypeButton}
		pollOpt          = testing.PollOptions{Timeout: 5 * time.Minute, Interval: 2 * time.Second}
	)

	// Click the add button at most once to prevent triggering
	// weird UI behaviors in Chrome Web Store.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Check if crxprt-2 is installed.
		if installed, err := ui.Exists(ctx, tconn, paramsLaunch); err != nil {
			return testing.PollBreak(err)
		} else if installed {
			return nil
		}

		if !addClicked {
			// If crxprt-2 is not installed, install it now.
			// Click on the add button, if it exists.
			if addButtonExists, err := ui.Exists(ctx, tconn, paramsAdd2Chrome); err != nil {
				return testing.PollBreak(err)
			} else if addButtonExists {
				addButton, err := ui.Find(ctx, tconn, paramsAdd2Chrome)
				if err != nil {
					return testing.PollBreak(err)
				}
				defer addButton.Release(ctx)

				if err := addButton.LeftClick(ctx); err != nil {
					return testing.PollBreak(err)
				}
				addClicked = true
			}
		}

		// Click on the confirm button, if it exists.
		if confirmButtonExists, err := ui.Exists(ctx, tconn, paramsAddApp); err != nil {
			return testing.PollBreak(err)
		} else if confirmButtonExists {
			confirmButton, err := ui.Find(ctx, tconn, paramsAddApp)
			if err != nil {
				return testing.PollBreak(err)
			}
			defer confirmButton.Release(ctx)

			if err := confirmButton.LeftClick(ctx); err != nil {
				return testing.PollBreak(err)
			}
		}
		return errors.New("crxprt-2 is still installing")
	}, &pollOpt); err != nil {
		return errors.Wrap(err, "failed to install crxprt-2")
	}
	return nil
}
