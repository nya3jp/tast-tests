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

	"github.com/mafredri/cdp/protocol/target"
	"golang.org/x/sync/errgroup"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui"
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

var connectedParams = ui.FindParams{Name: "Connected"}

// tabDetail define the detail of a Chrome tab in this test case
type tabDetail struct {
	URL, title string
	conn       *chrome.Conn
}

// prepare sets up the test case. It turns on the display, connecting to a
// Wi-Fi AP if it's disconnected.
func prepare(ctx context.Context, s *testing.State, cr *chrome.Chrome) (func(context.Context, *testing.State, *chrome.TestConn), error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API")
	}

	if err := power.TurnOnDisplay(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to turn on display")
	}

	var (
		ssid = s.RequiredVar("ui.cuj_wifissid")
		wpwd = s.RequiredVar("ui.cuj_wifipassword")
	)
	s.Log("Checking Wi-Fi connection via Settings app")

	s.Log("Launch network page")
	if _, err := ossettings.LaunchAtPage(ctx, tconn, nodewith.Name("Wi-Fi").Role(role.Button)); err != nil {
		return nil, errors.Wrap(err, "failed to launch Network page")
	}

	// after Settings page is opened, cleanup should close Settings page
	cleanup := func(ctx context.Context, s *testing.State, tconn *chrome.TestConn) {
		if err := apps.Close(ctx, tconn, apps.Settings.ID); err != nil {
			s.Fatal("Failed to close setting app: ", err)
		}
		if err := ash.WaitForAppClosed(ctx, tconn, apps.Settings.ID); err != nil {
			s.Fatal("Failed to close setting app: ", err)
		}
	}

	if err := enableWiFi(ctx, tconn); err != nil {
		return cleanup, errors.Wrap(err, "failed to enable Wi-Fi")
	}

	if err := ui.WaitUntilExists(ctx, tconn, connectedParams, time.Second*5); err != nil {
		if !errors.Is(err, ui.ErrNodeDoesNotExist) {
			return cleanup, errors.Wrap(err, "failed to check if the Wi-Fi AP is connected")
		}

		s.Log("Connecting to the Wi-Fi AP with SSID: ", ssid)
		apParams := ui.FindParams{
			Role:       ui.RoleTypeButton,
			Attributes: map[string]interface{}{"name": regexp.MustCompile(ssid + ",")},
		}
		if err = cuj.WaitAndClick(ctx, tconn, apParams, 15*time.Second); err != nil {
			return cleanup, errors.Wrap(err, "failed to find Wi-Fi AP")
		}
		if err = ui.WaitUntilExists(ctx, tconn, ui.FindParams{Name: "Join Wi-Fi network"}, 5*time.Second); err != nil {
			return cleanup, errors.Wrap(err, "failed to wait Wi-Fi dialog shows up")
		}

		kb, err := input.Keyboard(ctx)
		if err != nil {
			return cleanup, errors.Wrap(err, "failed to initialize keyboard")
		}
		defer kb.Close()

		if err = kb.Type(ctx, wpwd+"\n"); err != nil {
			return cleanup, errors.Wrap(err, "failed to type Wi-Fi password")
		}
		if err = ui.WaitUntilExists(ctx, tconn, connectedParams, 15*time.Second); err != nil {
			return cleanup, errors.Wrapf(err, "failed to connect to %s", ssid)
		}
	}

	s.Log("Wi-Fi is connected")
	return cleanup, nil
}

// Run runs the QuickCheckCUJ test. The lock is the function that suspends or
// locks the DUT. The record indicates if the lock function should be covered
// by metrics recorder.
func Run(ctx context.Context, s *testing.State, cr *chrome.Chrome, lock func(context.Context) error, record, tabletMode bool) *perf.Values {
	cleanup, err := prepare(ctx, s, cr)
	if err != nil {
		s.Fatal("Failed to prepare for QuickCheckCUJ")
	}

	if !record {
		if err := lock(ctx); err != nil {
			s.Fatal("Failed to lock or suspend DUT: ", err)
		}
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API")
	}

	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()
	defer func() {
		// cleanup of func: prepare()
		if cleanup != nil {
			cleanup(ctx, s, tconn)
		}
	}()

	screen, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get internal display info: ", err)
	}

	recorder, err := cuj.NewRecorder(ctx, tconn, cuj.MetricConfigs()...)
	if err != nil {
		s.Fatal("Failed to create a CUJ recorder: ", err)
	}
	defer recorder.Close(closeCtx)

	setBatteryNormal, err := setup.SetBatteryDischarge(ctx, 50)
	if err != nil {
		s.Fatal("Failed to set battery discharge: ", err)
	}
	defer setBatteryNormal(ctx)

	var diff time.Duration
	if err = recorder.Run(ctx, func(ctx context.Context) error {
		if record {
			if err := lock(ctx); err != nil {
				s.Fatal("Failed to lock or suspend DUT: ", err)
			}
		}
		if err = ui.WaitUntilExists(ctx, tconn, connectedParams, 10*time.Second); err != nil {
			return errors.Wrap(err, "waiting for Wi-Fi connected failed")
		}
		s.Log("Wi-Fi is connected")

		startTime := time.Now()

		// [R89 adjustment] directly use cr.NewConn() instead
		// if tabletMode {
		// 	startTime, err = cuj.LaunchAppFromHotseat(ctx, tconn, apps.Chrome)
		// 	if err != nil {
		// 		return errors.Wrap(err, "failed to launch Chrome")
		// 	}
		// } else {
		// 	startTime, err = cuj.LaunchAppFromShelf(ctx, tconn, apps.Chrome)
		// 	if err != nil {
		// 		return errors.Wrap(err, "failed to launch Chrome")
		// 	}
		// }

		diff = time.Since(startTime)

		tabs := []tabDetail{
			{title: `Inbox.*`, URL: "https://mail.google.com"},
			{title: `Calendar.*`, URL: "https://calendar.google.com/calendar/u/0/r/month"},
			{title: `Google\ News.*`, URL: "https://news.google.com"},
			{title: `Photos.*`, URL: "https://photos.google.com"},
		}

		// frist step of test scenario is open all tabs.
		for idx := range tabs {
			tab := &tabs[idx]
			if tab.conn, err = cr.NewConn(ctx, tab.URL, cdputil.WithNewWindow()); err != nil {
				return errors.Wrapf(err, "failed to open URL: %s", tab.URL)
			}
			defer tab.conn.Close()
			defer tab.conn.CloseTarget(closeCtx)
		}

		// after tabs are all opened, then rotate tablet device while browsing.
		if tabletMode {
			s.Log("Rotating the display")
			revert, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
			if err != nil {
				return errors.Wrap(err, "failed ensuring tablet mode")
			}
			defer revert(closeCtx)

			for _, rotation := range []display.RotationAngle{display.Rotate90, display.Rotate180, display.Rotate270, display.Rotate0} {
				if err = display.SetDisplayRotationSync(ctx, tconn, screen.ID, rotation); err != nil {
					return errors.Wrap(err, "failed rotating display")
				}
			}
		}

		// the test scenario here in this test case is rotate tablet device while browsing,
		// therefore, needs to wait after the rotation instead of after tabs are opened.
		g, encCtx := errgroup.WithContext(ctx)
		for idx := range tabs {
			g.Go(func() error {
				if err := webutil.WaitForQuiescence(encCtx, tabs[idx].conn, time.Minute); err != nil {
					return errors.Wrapf(err, "a tab is still loading [%s]", tabs[idx].URL)
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			s.Error("Some tabs are still loading: ", err)
		}

		s.Log("Minimize all the windows")
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

		if err := minimizeWindow(ctx, s, tconn, `Settings.*`); err != nil {
			s.Errorf("Couldn't minimize window [%s], error: %v", "Settings", err)
		} else {
			s.Log("Window: [Settings] is minimized")
		}

		return nil
	}); err != nil {
		s.Fatal("Failed to conduct the test scenario, or collect the histogram data: ", err)
	}

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "Browser.StartTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(diff.Milliseconds()))

	if err = recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to collect the data from the recorder: ", err)
	}

	if err = recorder.SaveHistograms(s.OutDir()); err != nil {
		s.Fatal("Failed to save histogram raw data from the recorder: ", err)
	}
	return pv
}

func minimizeWindow(ctx context.Context, s *testing.State, tconn *chrome.TestConn, tabtitle string) error {
	paramsWindow := ui.FindParams{
		Attributes: map[string]interface{}{"name": regexp.MustCompile(tabtitle)},
		Role:       ui.RoleTypeWindow,
	}
	paramsMinimize := ui.FindParams{
		Role:      ui.RoleTypeButton,
		Name:      "Minimize",
		ClassName: "FrameCaptionButton",
	}
	window, err := ui.StableFind(ctx, tconn, paramsWindow, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 1 * time.Second})
	if err != nil {
		return errors.Wrap(err, "failed to find window")
	}
	defer window.Release(ctx)
	node, err := window.DescendantWithTimeout(ctx, paramsMinimize, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find minimize button")
	}
	defer node.Release(ctx)
	if err = node.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click minimize button")
	}

	return nil
}

// WakeUpDuration reads dmesg, looking for the message about device resuming.
// It returns a channel of time duration and sends the value if the given
// context isn't cancelled.
func WakeUpDuration(ctx context.Context, s *testing.State) (chan time.Duration, error) {
	cmd := testexec.CommandContext(ctx, "dmesg", "--clear")
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return nil, errors.Wrap(err, "failed to clear log buffer")
	}

	cmd = testexec.CommandContext(ctx, "dmesg", "--follow")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start dmesg")
	}

	ch := make(chan time.Duration)
	go func() {
		defer out.Close()

		var (
			scanner = bufio.NewScanner(out)
			reg     = regexp.MustCompile("PM: resume of devices complete after ([0-9.]+) m?secs")
			msg     string
		)
		for scanner.Scan() {
			if msg = scanner.Text(); reg.MatchString(msg) {
				break
			}
		}

		ss := reg.FindStringSubmatch(msg)
		if len(ss) < 2 {
			s.Fatal("No resuming message was found")
		}

		f, err := strconv.ParseFloat(ss[1], 64)
		if err != nil {
			s.Fatalf("Failed to scan dmesg %q, error: %v", msg, err)
		}

		select {
		case <-ctx.Done():
		case ch <- time.Duration(f * float64(time.Millisecond)):
		}
	}()
	return ch, nil
}

func matchTargetTitle(title string) cdputil.TargetMatcher {
	return func(t *target.Info) bool { return t.Title == title }
}

func enableWiFi(ctx context.Context, tconn *chrome.TestConn) error {
	params := ui.FindParams{
		Attributes: map[string]interface{}{"name": regexp.MustCompile("Settings.*")},
		Role:       ui.RoleTypeRootWebArea,
	}

	ossRoot, err := ui.FindWithTimeout(ctx, tconn, params, time.Second*10)
	if err != nil {
		return errors.Wrap(err, "failed to get root node of os-settings window")
	}
	defer ossRoot.Release(ctx)

	paramsToggleBtn := ui.FindParams{
		Role:       ui.RoleTypeToggleButton,
		Attributes: map[string]interface{}{"name": regexp.MustCompile(regexp.QuoteMeta("Wi-Fi"))},
	}
	toggleButton, err := ossRoot.DescendantWithTimeout(ctx, paramsToggleBtn, 30*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to get toggle button node in os-settings window")
	}
	defer toggleButton.Release(ctx)

	if toggleButton.Checked != ui.CheckedStateTrue {
		if err := toggleButton.LeftClick(ctx); err != nil {
			return errors.Wrap(err, "failed to enable Wi-Fi")
		}
	}

	return nil
}
