// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package quickcheckcuj

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

// tabInfo define the detail of a Chrome tab in this test case
type tabInfo struct {
	url  string
	conn *chrome.Conn
}

// PauseMode defines how to pause the DUT during testing.
type PauseMode int

// Enums for PauseMode.
const (
	// Lock indicates to lock the screen during test.
	Lock PauseMode = iota
	// Suspend indicates to suspend the DUT during test.
	Suspend
)

// Run runs the QuickCheckCUJ2 test. The lock is the function that suspends or locks
// the DUT. The lockInRecorder flag indicates if the lock function should be executed
// inside metrics recorder.
func Run(ctx context.Context, s *testing.State, cr *chrome.Chrome, pauseMode PauseMode, tabletMode bool) *perf.Values {
	password := s.RequiredVar("ui.cuj_password")

	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API")
	}

	// Check and prepare wifi.
	performWifi := true
	ssid, ok1 := s.Var("ui.QuickCheckCUJ2_wifissid")
	wpwd, ok2 := s.Var("ui.QuickCheckCUJ2_wifipassword")
	if !ok1 || !ok2 {
		performWifi = false
		s.Log("Either WiFi SSID or password is not provided, and WiFi procedure will be skipped")
	}

	var wifi *shill.WifiManager
	if performWifi {
		if wifi, err = shill.NewWifiManager(ctx, nil); err != nil {
			s.Fatal("Failed to create shill Wi-Fi manager: ", err)
		}
		// Ensure wi-fi is enabled.
		if err := wifi.Enable(ctx, true); err != nil {
			s.Fatal("Failed to enable Wi-Fi: ", err)
		}
		s.Log("Wi-Fi is enabled")
		if err := wifi.ConnectAP(ctx, ssid, wpwd); err != nil {
			s.Fatalf("Failed to connect Wi-Fi AP %s: %v", ssid, err)
		}
		s.Logf("Wi-Fi AP %s is connected", ssid)
	}

	// Shorten the context to resume battery charging.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Set battery to discharge so the power consumption of the CUJ can be measured.
	setBatteryNormal, err := setup.SetBatteryDischarge(ctx, 50)
	if err != nil {
		s.Fatal("Failed to set battery discharge: ", err)
	}
	// It is important to call the deferred function setBatteryNormal() in
	// a separate context to make sure it has time to
	// run and the battery change can be set back to normal.
	defer setBatteryNormal(cleanupCtx)

	pv := perf.NewValues()

	// Execute Suspend function outside of recorder at the beginning because suspend will pause the
	// execution of the program and recorder and chrome needs to be reset.
	if pauseMode == Suspend {
		suspendSeconds := 40 // Tast runner might time out if suspend more than 60 seconds.
		// Use a shorter context to track wakeup duration.
		sCtx, sCancel := ctxutil.Shorten(ctx, 5*time.Second)
		defer sCancel()

		ch, err := trackWakeupDuration(sCtx)
		if err != nil {
			s.Fatal("Failed to tract wakeup time: ", err)
		}

		suspendStartTime := time.Now().UnixNano()
		if err = SuspendAndResume(ctx, cr, suspendSeconds); err != nil {
			s.Fatal("Failed to suspend the DUT: ", err)
		}
		select {
		case d, ok := <-ch:
			if !ok {
				s.Fatal("Wakeup time tracking returns no value")
			}
			s.Log("DUT wakeup time: ", d)
			pv.Set(perf.Metric{
				Name:      "QuickCheckCUJ.WakeUpTime",
				Unit:      "ms",
				Direction: perf.SmallerIsBetter,
			}, float64(d.Milliseconds()))

			resumeEndTime := time.Now().UnixNano()
			// Use unix nanoseconds to avoide monotonic clock of time.Since().
			sleepSeconds := float64(resumeEndTime-suspendStartTime) / 1e9
			s.Logf("DUT finished suspend and resume after %v seconds", sleepSeconds)
			if sleepSeconds > float64(suspendSeconds+3) {
				// Noticeable longer sleep time.
				s.Errorf("Suspend/resume is expected to finish in about %d seconds, but took %v seconds", suspendSeconds, sleepSeconds)
			}
		case <-ctx.Done():
			// This case should not happen because the trackWakeupDuration() uses
			// a shorter timeout value and the channel should have been closed already.
			s.Fatal("Failed to wait for wakeup time to be returned")
		}
		// After suspend/resume, all connections associated with the chrome.Chrome instance are invalid.
		// Reconnect to test API
		if tconn, err = cr.TestAPIConn(ctx); err != nil {
			s.Fatal("Failed to reconnect to Test API: ", err)
		}
	}

	// Shorten the context to cleanup recorder.
	cleanupCtx = ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	recorder, err := cuj.NewRecorder(ctx, tconn, cuj.MetricConfigs()...)
	if err != nil {
		s.Fatal("Failed to create a CUJ recorder: ", err)
	}
	defer recorder.Close(cleanupCtx)

	if pauseMode == Lock {
		// Lock the screen before recording the test.
		if err := LockScreen(ctx, tconn); err != nil {
			s.Fatal("Failed to lock screen")
		}

		defer func() {
			// Ensure that screen is unlocked even if the test fails.
			st, err := lockscreen.GetState(ctx, tconn)
			if err != nil {
				s.Error("Failed to get lockscreen state: ", err)
				return
			}
			if !st.Locked {
				return
			}
			if err := UnlockScreen(ctx, tconn, password); err != nil {
				s.Error("Failed unlock screen: ", err)
			}
		}()
	}

	// Create uiActionHandler at this point to make sure new tconn after suspend/resume is used.
	var uiActionHandler cuj.UIActionHandler
	if tabletMode {
		if uiActionHandler, err = cuj.NewTabletActionHandler(ctx, tconn); err != nil {
			s.Fatal("Failed to create tablet action handler: ", err)
		}
	} else {
		if uiActionHandler, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
			s.Fatal("Failed to create clamshell action handler: ", err)
		}
	}
	defer uiActionHandler.Close()

	var browserStartTime, totalElapsed time.Duration
	if err = recorder.Run(ctx, func(ctx context.Context) error {
		startTime := time.Now()

		// Execute lock function inside of recorder.
		if pauseMode == Lock {
			if err := UnlockScreen(ctx, tconn, password); err != nil {
				return errors.Wrap(err, "failed to lock and unlock screen")
			}
		}
		if performWifi {
			// Verify WiFi AP is re-connected after resume.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				c, err := wifi.Connected(ctx)
				if err != nil {
					return err
				}
				if !c {
					return errors.New("WiFi is not connected")
				}
				return nil
			}, &testing.PollOptions{Timeout: 30 * time.Second, Interval: 1 * time.Second}); err != nil {
				return errors.Wrap(err, "failed to re-connect WiFi after resume")
			}
			testing.ContextLog(ctx, "WiFi AP has been reconnected")
		}

		// Launch browser and track the elapsed time.
		if browserStartTime, err = cuj.GetBrowserStartTime(ctx, cr, tconn, tabletMode); err != nil {
			return errors.Wrap(err, "failed to launch Chrome")
		}
		testing.ContextLogf(ctx, "Browser start time %d ms", browserStartTime.Milliseconds())

		// Expecting 3 windows, first 2 windows with one tab and last window with 2 tabs.
		tabsInfo := [][]*tabInfo{{
			{url: "https://mail.google.com"},
		}, {
			{url: "https://calendar.google.com/calendar/u/0/r/month"},
		}, {
			{url: "https://news.google.com"},
			{url: "https://photos.google.com"},
		}}

		// Open tabs.
		for _, tabs := range tabsInfo {
			for tabIdx, tab := range tabs {
				defer func() {
					if tab.conn != nil {
						tab.conn.CloseTarget(ctx)
						tab.conn.Close()
						tab.conn = nil
					}
				}()

				if tab.conn, err = uiActionHandler.NewChromeTab(ctx, cr, tab.url, tabIdx == 0); err != nil {
					return errors.Wrapf(err, "failed to open URL: %s", tab.url)
				}
			}

			// Switch back to first tab.
			if len(tabs) > 1 {
				if err := uiActionHandler.SwitchToChromeTabByIndex(0)(ctx); err != nil {
					return errors.Wrap(err, "failed to switch back to first tab")
				}
			}
		}

		// After tabs are all opened, rotate tablet device while browsing,
		// and only do rotation on tablet mode
		if tabletMode {
			if err := rotate(ctx, tconn); err != nil {
				return errors.Wrap(err, "failed to rotate device's display")
			}
		}

		chromeApp, err := apps.ChromeOrChromium(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to check installed chrome browser")
		}

		scrollActions, err := uiActionHandler.ScrollChromePage(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to generate scroll actions")
		}

		// Switch windows/tabs and scroll the web page to measure the responsiveness.
		for idxWindow, tabs := range tabsInfo {
			switchFunc := uiActionHandler.SwitchToAppWindowByIndex(chromeApp.Name, idxWindow)
			for idxTab, tab := range tabs {
				if idxTab != 0 {
					switchFunc = uiActionHandler.SwitchToChromeTabByIndex(idxTab)
				}

				if err := switchFunc(ctx); err != nil {
					return errors.Wrap(err, "failed to switch between tabs")
				}
				if err := webutil.WaitForRender(ctx, tab.conn, 10*time.Second); err != nil {
					return errors.Wrapf(err, "failed to wait for finish render [%s]", tab.url)
				}
				// Wait each page to finish loading (to see if the network connection works)
				if err := webutil.WaitForQuiescence(ctx, tab.conn, time.Minute); err != nil {
					return errors.Wrapf(err, "a tab is still loading [%s]", tab.url)
				}

				for _, scroll := range scrollActions {
					if err := scroll(ctx); err != nil {
						return errors.Wrap(err, "failed to scroll page")
					}
				}
			}
		}

		// Total time used from beginning to load all pages.
		totalElapsed = time.Since(startTime)
		testing.ContextLogf(ctx, "All page loaded, %d ms elapsed", totalElapsed.Milliseconds())

		if err := uiActionHandler.MinimizeAllWindow()(ctx); err != nil {
			return errors.Wrap(err, "failed to minimize all window: ")
		}

		return nil
	}); err != nil {
		s.Fatal("Failed to conduct the test scenario, or collect the histogram data: ", err)
	}

	pv.Set(perf.Metric{
		Name:      "Browser.StartTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(browserStartTime.Milliseconds()))

	pv.Set(perf.Metric{
		Name:      "QuickCheckCUJ.ElapsedTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(totalElapsed.Milliseconds()))

	if err = recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to collect the data from the recorder: ", err)
	}
	// We don't do pv.Save(), but will return and let the test case handle it.
	if err = recorder.SaveHistograms(s.OutDir()); err != nil {
		s.Fatal("Failed to save histogram raw data from the recorder: ", err)
	}
	return pv
}

// trackWakeupDuration reads dmesg, looking for the device resume complete message, and captures the
// time used to wake up the device.
// It returns a channel of time duration, which will send the resume time found from the log.
func trackWakeupDuration(ctx context.Context) (chan time.Duration, error) {
	cmd := testexec.CommandContext(ctx, "dmesg", "--clear")
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrap(err, `failed to clear log buffer with "dmesg --clear"`)
	}

	cmd = testexec.CommandContext(ctx, "dmesg", "--follow")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, `failed to start "dmesg --follow" command`)
	}

	ch := make(chan time.Duration, 1)
	go func() {
		defer func() {
			// Release cmd resources.
			out.Close()
			cmd.Kill()
			cmd.Wait()

			close(ch)
		}()

		// The Suspend/Resume procedure will have these steps:
		// 1. Trigger suspend with powerd_dbus_suspend command.
		// 2. Expect the following logs in sequence:
		//		2.1 NOTICE powerd_suspend[21209]: Finalizing suspend
		//		2.2 INFO kernel: [ 1362.975945] PM: suspend entry (deep)
		//		2.3 INFO kernel: [ 1362.975955] PM: Syncing filesystems ...
		// 		2.4 DEBUG kernel: [ 1363.520587] PM: Preparing system for sleep (deep)
		// 		2.5 DEBUG kernel: [ 1363.608433] PM: Suspending system (deep)
		//		2.6 DEBUG kernel: [ 1363.737905] PM: suspend of devices complete after 128.995 msecs
		// 		2.7 DEBUG kernel: [ 1363.740330] PM: late suspend of devices complete after 2.419 msecs
		// 		2.8 DEBUG kernel: [ 1363.765000] PM: noirq suspend of devices complete after 24.615 msecs
		//		Optional 2.9.1 INFO kernel: [ 1363.766674] PM: Saving platform NVS memory
		//		Optional 2.9.2 PM: suspend-to-idle
		//		Optional 2.10 INFO kernel: [ 1363.774959] PM: Restoring platform NVS memory
		//		2.11 DEBUG kernel: [ 1363.775205] PM: Timekeeping suspended for 39.208 seconds
		//		Optional 2.11.2: PM: resume from suspend-to-idle
		//		2.12 DEBUG kernel: [ 1363.831897] PM: noirq resume of devices complete after 25.953 msecs
		//		2.13 DEBUG kernel: [ 1363.858700] PM: early resume of devices complete after 26.677 msecs
		//		2.14 DEBUG kernel: [ 1364.116961] PM: resume of devices complete after 258.257 msecs
		//		2.15 DEBUG kernel: [ 1364.117403] PM: Finishing wakeup.
		//		2.16 INFO kernel: [ 1364.130740] PM: suspend exit
		//		2.17 NOTICE powerd_suspend[21209]: wake source: PM1_STS: WAK RTC BMSTATUS
		//		2.18 NOTICE powerd_suspend[21209]: Resume finished

		// Find the timestamp of the first message shown from the following slice (step 2.10, 2.11, 2.11.2)
		// as the wakeup start time.
		msgStart := []*regexp.Regexp{
			regexp.MustCompile(`\[\s*(\d+\.\d+)\] PM: Restoring platform NVS memory`),
			regexp.MustCompile(`\[\s*(\d+\.\d+)\] PM: Timekeeping suspended for .* seconds`),
			regexp.MustCompile(`\[\s*(\d+\.\d+)\] PM: resume from suspend-to-idle`),
		}
		msgSuspendExist := regexp.MustCompile(`\[\s*(\d+\.\d+)\] PM: suspend exit`)

		var resumeStart, resumeExit float64
		scanner := bufio.NewScanner(out)

		// Scan output util it returns false, or matched pattern is found.
		for scanner.Scan() {
			text := scanner.Text()
			if resumeStart == 0.0 {
				for _, msg := range msgStart {
					if ss := msg.FindStringSubmatch(text); ss != nil {
						resumeStart, err = strconv.ParseFloat(ss[1], 64)
						if err != nil {
							testing.ContextLogf(ctx, "Failed to get wakeup start timestamp from %q: %v", text, err)
							return
						}
						testing.ContextLog(ctx, "Wakeup start timestamp: ", resumeStart)
						break
					}
				}
			}
			if ss := msgSuspendExist.FindStringSubmatch(text); ss != nil {
				resumeExit, err = strconv.ParseFloat(ss[1], 64)
				if err != nil {
					testing.ContextLogf(ctx, "Failed to get wakeup exit timestamp from %q: %v", text, err)
					return
				}
				testing.ContextLog(ctx, "Wakeup exit timestamp: ", resumeExit)
				if resumeStart == 0 {
					testing.ContextLog(ctx, "Got the wakeup exit timestamp but didn't get wakeup start timestamp")
					return
				}
				if resumeStart > 0 && resumeExit > 0 {
					ch <- time.Duration(math.Round((resumeExit-resumeStart)*1000)) * time.Millisecond
					break
				}
			}
		}
	}()

	return ch, nil
}

// SuspendAndResume suspends the ChromeOS and then wakes it up.
// After calling this method, all connections associated with the current browser session are no longer valid and
// need to be re-established.
func SuspendAndResume(ctx context.Context, cr *chrome.Chrome, suspendSeconds int) error {
	testing.ContextLogf(ctx, "Start a DUT suspend of %d seconds", suspendSeconds)
	cmd := testexec.CommandContext(ctx, "powerd_dbus_suspend", fmt.Sprintf("--wakeup_timeout=%d", suspendSeconds))
	if err := cmd.Run(); err != nil {
		return err
	}

	testing.ContextLog(ctx, "DUT resumes from suspend")
	// After resume from suspend, the connection to browser session needs to be re-established.
	return cr.Reconnect(ctx)
}

// LockScreen locks the screen.
func LockScreen(ctx context.Context, tconn *chrome.TestConn) error {
	const lockTimeout = 30 * time.Second

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create keyboard event writer")
	}
	defer kb.Close()

	const accel = "Search+L"
	testing.ContextLogf(ctx, "Locking screen via %q", accel)
	if err := kb.Accel(ctx, accel); err != nil {
		return errors.Wrapf(err, "typing %q failed", accel)
	}
	testing.ContextLog(ctx, "Waiting for Chrome to report that screen is locked")
	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, lockTimeout); err != nil {
		return errors.Wrapf(err, "waiting for screen to be locked failed (last status %+v)", st)
	}

	return nil
}

// UnlockScreen unlocks the screen with the given keyboard writer.
func UnlockScreen(ctx context.Context, tconn *chrome.TestConn, password string) error {
	const goodAuthTimeout = 30 * time.Second

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create keyboard event writer")
	}
	defer kb.Close()

	testing.ContextLog(ctx, "Unlocking screen by typing password")
	if err := kb.Type(ctx, password+"\n"); err != nil {
		return errors.Wrap(err, "typing correct password failed")
	}
	testing.ContextLog(ctx, "Waiting for Chrome to report that screen is unlocked")
	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return !st.Locked }, goodAuthTimeout); err != nil {
		return errors.Wrapf(err, "waiting for screen to be unlocked failed (last status %+v)", st)
	}
	return nil
}

func rotate(ctx context.Context, tconn *chrome.TestConn) error {
	screen, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		testing.ContextLog(ctx, "Failed to get internal display info: ", err)
	}
	testing.ContextLog(ctx, "Rotating the display")

	for _, rotation := range []display.RotationAngle{display.Rotate90, display.Rotate180, display.Rotate270, display.Rotate0} {
		if err = display.SetDisplayRotationSync(ctx, tconn, screen.ID, rotation); err != nil {
			return errors.Wrap(err, "failed rotating display")
		}
	}

	return nil
}
