// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package quickcheckcuj

import (
	"bufio"
	"context"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

var connectedNodeFinder = nodewith.Name("Connected").First()

// tabDetail define the detail of a Chrome tab in this test case
type tabDetail struct {
	URL, title string
	conn       *chrome.Conn
}

// prepareWiFi sets up the Wi-Fi for the test. It connects to a Wi-Fi AP if there is no WiFi connection.
func prepareWiFi(ctx context.Context, s *testing.State, tconn *chrome.TestConn) error {
	ssid := s.RequiredVar("ui.cuj_wifissid")
	wpwd := s.RequiredVar("ui.cuj_wifipassword")

	s.Log("Checking Wi-Fi connection via Settings app")
	if _, err := ossettings.Launch(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to launch OS setting")
	}

	// Ensure the wi-fi is enabled to that the page can be navigate to further page
	if err := enableWiFi(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to enable Wi-Fi")
	}
	s.Log("Wi-Fi enabled")

	ui := uiauto.New(tconn)

	// Navigate to wifi page by click "Wi-Fi" button
	wifiBtn := nodewith.Name("Wi-Fi").Role(role.Button)
	if err := ui.IfSuccessThen(ui.WaitUntilExists(wifiBtn), ui.LeftClick(wifiBtn))(ctx); err != nil {
		return errors.Wrap(err, "failed to find Wi-Fi AP")
	}

	if err := ui.WaitUntilExists(connectedNodeFinder)(ctx); err != nil {
		s.Log("Wi-Fi is not connected: ", err)
	} else {
		s.Log("Wi-Fi is connected")
		return nil
	}

	s.Logf("Connecting to Wi-Fi (SSID: %s)", ssid)
	wifiApNode := nodewith.Role(role.Button).NameRegex(regexp.MustCompile(regexp.QuoteMeta(ssid))).First()
	if err := ui.IfSuccessThen(ui.WaitUntilExists(wifiApNode), ui.LeftClick(wifiApNode))(ctx); err != nil {
		return errors.Wrap(err, "failed to find Wi-Fi AP")
	}

	joinWifiNode := nodewith.Name("Join Wi-Fi network").First()
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		foundConnected := ui.Exists(connectedNodeFinder)(ctx) == nil
		foundJoinWifi := ui.Exists(joinWifiNode)(ctx) == nil

		if foundConnected {
			// The AP could be connected automatically without asking password.
			// e.g. the wi-fi is just simply turned off with AP info still remembered.
			return nil
		} else if foundJoinWifi {
			// Needs to type the password to join this wifi ap
			kb, err := input.Keyboard(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to initialize keyboard")
			}
			defer kb.Close()

			if err = kb.Type(ctx, wpwd+"\n"); err != nil {
				return errors.Wrap(err, "failed to type Wi-Fi password")
			}

			return errors.New("password typed, check connected again")
		}

		return errors.New("expected nodes not appear, check again")
	}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
		return errors.Wrapf(err, "failed to connect to Wi-Fi network %q", ssid)
	}

	s.Log("Wi-Fi is connected")
	return nil
}

// PauseMode defines how to pause the DUT during testing.
type PauseMode int

// Enums for PauseMode.
const (
	Lock PauseMode = iota
	Suspend
)

// Run runs the QuickCheckCUJ test. The lock is the function that suspends or locks
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

	// Check and prepare wifi. WiFi settings will be left open for later checking logic.
	if err := prepareWiFi(ctx, s, tconn); err != nil {
		s.Fatal("Failed to prepare for QuickCheckCUJ: ", err)
	}
	// Close WiFi Settings page
	defer apps.Close(ctx, tconn, apps.Settings.ID)

	setBatteryNormal, err := setup.SetBatteryDischarge(ctx, 50)
	if err != nil {
		s.Fatal("Failed to set battery discharge: ", err)
	}
	defer setBatteryNormal(ctx)

	pv := perf.NewValues()

	// Execute Suspend function outside of recorder at the beginning because suspend will pause the
	// execution of the program and recorder and chrome needs to be reset.
	if pauseMode == Suspend {
		// Use a shorter context to track wakeup duration.
		sCtx, sCancel := ctxutil.Shorten(ctx, 2*time.Second)
		defer sCancel()
		ch, err := trackWakeupDuration(sCtx)
		if err != nil {
			s.Fatal("Failed to tract wakeup time: ", err)
		}

		if err := SuspendAndResume(ctx, cr); err != nil {
			s.Fatal("Failed to suspend the DUT: ", err)
		}

		select {
		case d, ok := <-ch:
			if !ok {
				s.Fatal("Wakeup time tracking returns no value")
			}
			pv.Set(perf.Metric{
				Name:      "QuickCheckCUJ.WakeUpTime",
				Unit:      "ms",
				Direction: perf.SmallerIsBetter,
			}, float64(d.Milliseconds()))
		case <-ctx.Done():
			// This case should not happen because the trackWakeupDuration() uses
			// a shorter timeout value and the channel should have been closed already.
			s.Fatal("Failed to wait for wakeup time to be returned")
		}

		// tconn has to be reconnected.
		tconn, err = cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to connect Test API after resume")
		}
	}

	recorder, err := cuj.NewRecorder(ctx, tconn, cuj.MetricConfigs()...)
	if err != nil {
		s.Fatal("Failed to create a CUJ recorder: ", err)
	}
	defer recorder.Close(ctx)

	if pauseMode == Lock {
		// Lock the screen before record the test.
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

	pollOpt := testing.PollOptions{Timeout: time.Minute, Interval: time.Second}
	ui := uiauto.New(tconn)

	var browserStartDuration, totalElapsed time.Duration
	// Use a shortened context to run recorder to allow cleanup.
	runCtx, runCancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer runCancel()
	if err = recorder.Run(runCtx, func(ctx context.Context) error {
		startTime := time.Now()

		// Execute lock function inside of recorder.
		if pauseMode == Lock {
			if err := UnlockScreen(ctx, tconn, password); err != nil {
				s.Fatal("Failed to lock and unlock screen: ", err)
			}
		}

		// It might took longer to reconnect to wifi after unlock or wakeup
		if err = ui.WithPollOpts(pollOpt).WaitUntilExists(connectedNodeFinder)(ctx); err != nil {
			return errors.Wrap(err, "waiting for Wi-Fi connected failed")
		}
		s.Log("Wi-Fi is connected")

		// Launch browser and track the elapsed time.
		browserStartTime := time.Now()
		if tabletMode {
			startTime, err = cuj.LaunchAppFromHotseat(ctx, tconn, "Google Chrome")
			if err != nil {
				return errors.Wrap(err, "failed to launch Chrome")
			}
		} else {
			startTime, err = cuj.LaunchAppFromShelf(ctx, tconn, "Google Chrome")
			if err != nil {
				return errors.Wrap(err, "failed to launch Chrome")
			}
		}
		browserStartDuration = time.Since(browserStartTime)
		s.Log("Browser start ms: ", browserStartDuration.Milliseconds())

		tabs := []tabDetail{
			{title: `Inbox.*`, URL: "https://mail.google.com"},
			{title: `Calendar.*`, URL: "https://calendar.google.com/calendar/u/0/r/month"},
			{title: `Google\ News.*`, URL: "https://news.google.com"},
			{title: `Photos.*`, URL: "https://photos.google.com"},
		}

		// First step of test scenario is open all tabs.
		for idx := range tabs {
			tab := &tabs[idx]
			if idx == 0 {
				if tab.conn, err = cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://newtab/")); err != nil {
					return errors.Wrap(err, "failed to find new tab")
				}
				if err = tab.conn.Navigate(ctx, tab.URL); err != nil {
					return errors.Wrapf(err, "failed navigating to %s", tab.URL)
				}
			} else {
				if tab.conn, err = cr.NewConn(ctx, tab.URL, cdputil.WithNewWindow()); err != nil {
					return errors.Wrapf(err, "failed to open URL: %s", tab.URL)
				}
			}
			defer tab.conn.Close()
			defer tab.conn.CloseTarget(ctx)
		}

		// After tabs are all opened, rotate tablet device while browsing.
		if tabletMode {
			screen, err := display.GetInternalInfo(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get internal display info: ", err)
			}

			s.Log("Rotating the display")
			for _, rotation := range []display.RotationAngle{display.Rotate90, display.Rotate180, display.Rotate270, display.Rotate0} {
				if err = display.SetDisplayRotationSync(ctx, tconn, screen.ID, rotation); err != nil {
					return errors.Wrap(err, "failed rotating display")
				}
			}
		}

		kb, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to create keyboard event writer")
		}
		defer kb.Close()

		// Switch windows for measure the responsiveness
		// and wait each window finish loading (also see if the wifi connection works)
		for idx := range tabs {
			if err := switchWindow(ctx, len(tabs), kb); err != nil {
				return errors.Wrap(err, "failed to switch between windows")
			}
			if err := webutil.WaitForRender(ctx, tabs[idx].conn, time.Minute); err != nil {
				s.Fatal("Failed to wait for finish render: ", err)
				return errors.Wrapf(err, "ailed to wait for finish render [%s]", tabs[idx].URL)
			}
			if err := webutil.WaitForQuiescence(ctx, tabs[idx].conn, time.Minute); err != nil {
				return errors.Wrapf(err, "a tab is still loading [%s]", tabs[idx].URL)
			}
		}

		// Total time used from beginning to load all pages.
		totalElapsed = time.Since(startTime)
		s.Log("Total Elapsed ms: ", totalElapsed.Milliseconds())

		s.Log("Minimize all the windows")
		if tabletMode {
			tsew, err := input.Touchscreen(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to get the touch screen")
			}
			defer tsew.Close()

			stw, err := tsew.NewSingleTouchWriter()
			if err != nil {
				return errors.Wrap(err, "failed to get the single touch event writer")
			}
			defer stw.Close()

			orientation, err := display.GetOrientation(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to obtain the display rotation")
			}

			if err = tsew.SetRotation(-orientation.Angle); err != nil {
				return errors.Wrap(err, "failed to set rotation")
			}

			s.Log("Drag to show homescreen")
			if err := ash.DragToShowHomescreen(ctx, tsew.Width(), tsew.Height(), stw, tconn); err != nil {
				return errors.Wrap(err, "failed to show homescreen")
			}
			if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfShownHomeLauncher); err != nil {
				return errors.Wrap(err, "hotseat is in an unexpected state")
			}
		} else {
			// last opened one should be on the topest layer, so iterate reversely
			for idx := len(tabs) - 1; idx >= 0; idx-- {
				tab := &tabs[idx]
				if err := minimizeWindow(ctx, s, tconn, tab.title); err != nil {
					s.Errorf("Couldn't minimize window [%s], error: %v", tab.URL, err)
				} else {
					s.Logf("Window: [%s] is minimized", tab.URL)
				}
				// a short pause for make this clicking action more human-like.
				// and since this is the last action of test case,
				// this makes tester able to see windows are been minimized authentically.
				testing.Sleep(ctx, 500*time.Millisecond)
			}
		}

		return nil
	}); err != nil {
		s.Fatal("Failed to conduct the test scenario, or collect the histogram data: ", err)
	}

	pv.Set(perf.Metric{
		Name:      "Browser.StartTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(browserStartDuration.Milliseconds()))

	pv.Set(perf.Metric{
		Name:      "QuickCheckCUJ.ElapsedTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(totalElapsed.Milliseconds()))

	if err = recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to collect the data from the recorder: ", err)
	}
	// We don't do pv.Save(), but will return it to let the test case to handle it.

	if err = recorder.SaveHistograms(s.OutDir()); err != nil {
		s.Fatal("Failed to save histogram raw data from the recorder: ", err)
	}
	return pv
}

func minimizeWindow(ctx context.Context, s *testing.State, tconn *chrome.TestConn, tabtitle string) error {
	pollOpt := testing.PollOptions{Timeout: time.Minute, Interval: time.Second}
	ui := uiauto.New(tconn)

	windowNode := nodewith.NameRegex(regexp.MustCompile(tabtitle)).Role(role.Window).First()
	minimizeBtn := nodewith.Name("Minimize").Role(role.Button).ClassName("FrameCaptionButton").Ancestor(windowNode)

	if err := ui.WaitUntilExists(windowNode)(ctx); err != nil {
		return errors.Wrap(err, "failed to find window")
	}

	if err := uiauto.Combine("Find minimize button under window and click it",
		ui.WithPollOpts(pollOpt).WaitUntilExists(minimizeBtn),
		ui.LeftClick(minimizeBtn),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to click minimize button")
	}

	return nil
}

func enableWiFi(ctx context.Context, tconn *chrome.TestConn) error {
	pollOpt := testing.PollOptions{Timeout: time.Minute, Interval: time.Second}
	ui := uiauto.New(tconn).WithPollOpts(pollOpt)

	settingsPageNode := nodewith.NameRegex(regexp.MustCompile("Settings.*")).Role(role.RootWebArea).First()
	if err := ui.WaitUntilExists(settingsPageNode)(ctx); err != nil {
		return errors.Wrap(err, "failed to find os-settings window")
	}

	name := regexp.MustCompile(regexp.QuoteMeta("Wi-Fi"))
	toggleButtonNode := nodewith.NameRegex(name).Role(role.ToggleButton).FinalAncestor(settingsPageNode).First()
	if err := ui.WaitUntilExists(toggleButtonNode)(ctx); err != nil {
		return errors.Wrap(err, "failed to find toggle button")
	}

	testing.Poll(ctx, func(ctx context.Context) error {
		info, err := ui.Info(ctx, toggleButtonNode)
		if err != nil {
			return errors.Wrap(err, "failed to check toggle button checked status")
		}

		if info.Checked == checked.False {
			if err := ui.LeftClick(toggleButtonNode)(ctx); err != nil {
				return errors.Wrap(err, "failed to enable Wi-Fi")
			}
		} else {
			return nil
		}

		return errors.Wrap(err, "toggle button just clicked, check again")
	}, &pollOpt)

	return nil
}

// trackWakeupDuration reads dmesg, looking for the device resume complete message and capture the
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
		defer close(ch)

		scanner := bufio.NewScanner(out)
		reg := regexp.MustCompile("PM: resume of devices complete after ([0-9.]+) m?secs")
		// Scan output util it returns false, or matched pattern is found.
		for scanner.Scan() {
			ss := reg.FindStringSubmatch(scanner.Text())
			if ss == nil {
				continue
			}
			f, err := strconv.ParseFloat(ss[1], 64)
			if err != nil {
				testing.ContextLogf(ctx, "Failed to convert resume time %q to float: %v", ss[1], err)
				return
			}
			ch <- time.Duration(f * float64(time.Millisecond))
			break
		}
	}()
	return ch, nil
}

// SuspendAndResume suspends the ChromeOS and then wakes it up.
func SuspendAndResume(ctx context.Context, cr *chrome.Chrome) error {
	// Suspend 40 seconds. Tast runner might time out if suspend more than 60 seconds.
	testing.ContextLog(ctx, "Start a DUT suspend of 40 seconds")
	cmd := testexec.CommandContext(ctx, "powerd_dbus_suspend", "--wakeup_timeout=40")
	if err := cmd.Run(); err != nil {
		return err
	}
	testing.ContextLog(ctx, "DUT resumes from suspend")
	// Restore the current chrome session.
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

func switchWindow(ctx context.Context, numWindows int, kw *input.KeyboardEventWriter) error {
	// TODO: tablet support
	if kw != nil {
		if err := kw.AccelPress(ctx, "Alt"); err != nil {
			return errors.Wrap(err, "failed to execute key event")
		}
		for i := 1; i < numWindows; i++ {
			if err := testing.Sleep(ctx, 200*time.Millisecond); err != nil {
				return errors.Wrap(err, "failed to sleep")
			}
			if err := kw.AccelPress(ctx, "Tab"); err != nil {
				return errors.Wrap(err, "failed to execute key event")
			}
			if err := testing.Sleep(ctx, 200*time.Millisecond); err != nil {
				return errors.Wrap(err, "failed to sleep")
			}
			if err := kw.AccelRelease(ctx, "Tab"); err != nil {
				return errors.Wrap(err, "failed to execute key event")
			}
		}
		if err := kw.AccelRelease(ctx, "Alt"); err != nil {
			return errors.Wrap(err, "failed to execute key event")
		}
	}

	return nil
}
