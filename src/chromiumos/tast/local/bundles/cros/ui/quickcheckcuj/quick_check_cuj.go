// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package quickcheckcuj

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
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
	setBatteryNormal, err := cuj.SetBatteryDischarge(ctx, 50)
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
		// sleepTime is the actual sleep time. The whole suspend procedure takes more time.
		sleepTime := 15
		earliestResumeEndTime := time.Now().Add(time.Duration(sleepTime) * time.Second)

		if err := suspendAndResume(ctx, cr, sleepTime); err != nil {
			s.Fatal("Failed to suspend the DUT: ", err)
		}

		wakeupDuration, err := readWakeupDuration(ctx, earliestResumeEndTime)
		if err != nil {
			s.Fatal("Failed to read wakeup time: ", err)
		}

		d := time.Duration(wakeupDuration*1000) * time.Millisecond
		s.Log("DUT wakeup time: ", d)
		pv.Set(perf.Metric{
			Name:      "QuickCheckCUJ.WakeUpTime",
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
		}, float64(d.Milliseconds()))

		// After suspend/resume, all connections associated with the chrome.Chrome instance are invalid.
		// Reconnect to test API
		if tconn, err = cr.TestAPIConn(ctx); err != nil {
			s.Fatal("Failed to reconnect to Test API: ", err)
		}
	}

	// Shorten the context to cleanup recorder.
	cleanupRecorderCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	recorder, err := cuj.NewRecorder(ctx, tconn, cuj.MetricConfigs()...)
	if err != nil {
		s.Fatal("Failed to create a CUJ recorder: ", err)
	}
	defer recorder.Close(cleanupRecorderCtx)

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
			pollOpts := testing.PollOptions{Timeout: 30 * time.Second, Interval: 1 * time.Second}
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				c, err := wifi.Connected(ctx)
				if err != nil {
					return err
				}
				if !c {
					return errors.New("WiFi is not connected")
				}
				return nil
			}, &pollOpts); err != nil {
				return errors.Wrapf(err, "failed to re-connect WiFi after resume within %v", pollOpts.Timeout)
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

			// Switch back to the first tab.
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
			switchDesc := "windows"
			for idxTab, tab := range tabs {
				if idxTab != 0 {
					switchFunc = uiActionHandler.SwitchToChromeTabByIndex(idxTab)
					switchDesc = "tabs"
				}

				if err := switchFunc(ctx); err != nil {
					return errors.Wrapf(err, "failed to switch between %s", switchDesc)
				}
				if err := webutil.WaitForRender(ctx, tab.conn, 10*time.Second); err != nil {
					return errors.Wrapf(err, "failed to wait for finish render [%s]", tab.url)
				}
				// Wait each page to finish loading (to see if the network connection works).
				timeout := time.Minute
				if err := webutil.WaitForQuiescence(ctx, tab.conn, timeout); err != nil {
					return errors.Wrapf(err, "a tab is still loading [%s] after %v", tab.url, timeout)
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

// suspendAndResume calls powerd_dbus_suspend command to suspend the system and lets it
// stay sleep for the given duration and then wake up.
func suspendAndResume(ctx context.Context, cr *chrome.Chrome, sleepTime int) error {
	// The actual time used to suspend and weekup the system is:
	// 		(time to suspend the system) + (sleep time) + (time to wakeup the system)
	// Tast runner might time out if DUT is inaccessible for more than 60 seconds.
	// We allow 30-second maximum sleep time, trying to keep the total suspend/wakeup time
	// under 1 minute.

	const maxSleepTime = 30
	if sleepTime > maxSleepTime {
		return errors.Errorf("suspend time should less than %d seconds", maxSleepTime)
	}

	// timeout, according to powerd_dbus_suspend help page, defines how long to wait for
	// a resume signal in seconds. We add 20 seconds to maxSleepTime to ensure the command
	// will exit if the whole suspend/wakeup procedure couldn't trigger a resume signal for
	// any reason within this time.
	timeout := maxSleepTime + 20

	// Read wakeup count here to prevent suspend retries, which happens without user input.
	wakeupCount, err := ioutil.ReadFile("/sys/power/wakeup_count")
	if err != nil {
		return errors.Wrap(err, "failed to read wakeup count before suspend")
	}

	cmd := testexec.CommandContext(
		ctx,
		"powerd_dbus_suspend",
		"--disable_dark_resume=true",
		fmt.Sprintf("--timeout=%d", timeout),
		fmt.Sprintf("--wakeup_count=%s", strings.Trim(string(wakeupCount), "\n")),
		fmt.Sprintf("--suspend_for_sec=%d", sleepTime),
	)
	testing.ContextLogf(ctx, "Start a DUT suspend of %d seconds: %s", sleepTime, cmd.Args)

	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "powerd_dbus_suspend failed to properly suspend")
	}

	testing.ContextLog(ctx, "DUT resumes from suspend")
	return cr.Reconnect(ctx)
}

// readWakeupDuration reads and calculates the wakeup duration from last_resume_timings file.
// The file's modification time must be newer than the earliestModTime to ensure the file has been updated
// by a successful suspend/wakeup.
func readWakeupDuration(ctx context.Context, earliestModTime time.Time) (float64, error) {
	const (
		lastResumeTimingsFile = "/run/power_manager/root/last_resume_timings"

		// suspendTotalTime is the time used to wait for suspend procedure to generate the
		// last_resume_timings file. In case of suspending failure, the DUT might retry
		// multiple times until it succeeds.
		suspendTotalTime = 2 * time.Minute
	)

	// Wait until the suspend procedure successfully generates the last_resume_timings with a
	// newer timestamp.
	pollOpts := testing.PollOptions{Timeout: suspendTotalTime, Interval: time.Second}
	if err := testing.Poll(ctx, func(c context.Context) error {
		fState, err := os.Stat(lastResumeTimingsFile)
		if err != nil {
			if os.IsNotExist(err) {
				return errors.New("file doesn't exist")
			}
			return testing.PollBreak(errors.Wrap(err, "failed to check file state"))
		}
		if !fState.ModTime().After(earliestModTime) {
			return errors.New("last_resume_timings file hasn't been updated")
		}
		return nil
	}, &pollOpts); err != nil {
		return 0.0, errors.Wrapf(err, "failed to check existence of a new last_resume_timings file within %v", pollOpts.Timeout)
	}

	b, err := ioutil.ReadFile(lastResumeTimingsFile)
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to read last_resume_timings file")
	}

	// The content of /run/power_manager/root/last_resume_timings should be as follows:
	// start_suspend_time = 183.825542
	// end_suspend_time = 184.213222
	// start_resume_time = 184.248745
	// end_resume_time = 185.480335
	// cpu_ready_time = 184.837355
	//
	// We'll use `start_resume_time` and `end_resume_time` to get the wakeup duration.
	lines := []*regexp.Regexp{
		regexp.MustCompile(`start_resume_time\s*=\s*(\d+\.\d+)`),
		regexp.MustCompile(`end_resume_time\s*=\s*(\d+\.\d+)`),
	}
	timestamps := []float64{0.0, 0.0} // start and end timestamp extracted from the file.
	for i, line := range lines {
		if ss := line.FindStringSubmatch(string(b)); ss != nil {
			timestamp, err := strconv.ParseFloat(ss[1], 64)
			if err != nil {
				return 0.0, errors.Wrapf(err, "failed to get timestamp for %v", line)
			}
			timestamps[i] = timestamp
		}
	}

	testing.ContextLog(ctx, "Resume start and end timestamps: ", timestamps)

	if timestamps[0] == 0.0 || timestamps[1] == 0.0 {
		return 0.0, errors.New("failed to find resume start or end timestamps")
	}
	return timestamps[1] - timestamps[0], nil
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
