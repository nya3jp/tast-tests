// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/gmail"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/remote/chameleon"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TC07S1ExtendedDisplayCUJ,
		Desc:         "Measures the performance of video entertainment with extended display CUJ",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"android_p", "chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      3 * time.Minute,
		Vars: []string{
			"ui.cuj_username",
			"ui.cuj_password",
			"chameleon",
		},
		Pre: cuj.LoginKeepState(),
	})
}

const (
	yt4kVideoURL = "https://www.youtube.com/watch?v=LXb3EKWsInQ"
	gmailURL     = "https://mail.google.com"

	uiTimeout = time.Second * 30
	sleepTime = time.Second * 5
)

func TC07S1ExtendedDisplayCUJ(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(cuj.PreKeepData).Chrome
	a := s.PreValue().(cuj.PreKeepData).ARC
	loginTime := s.PreValue().(cuj.PreKeepData).LoginTime
	chameleonAddr := s.RequiredVar("chameleon")
	tabletMode := false

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer cuj.CloseAllWindows(ctx, tconn)

	che, err := chameleon.New(ctx, chameleonAddr)
	if err != nil {
		s.Fatal("Failed to connect to chameleon board: ", err)
	}

	che.Plug(ctx, 3)
	defer che.Unplug(ctx, 3)

	// Wait DUT detect external display
	if err := che.WaitVideoInputStable(ctx, 3, 10*time.Second); err != nil {
		s.Fatal("Failed to plug external display: ", err)
	}

	displayInfos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get display info: ", err)
	}

	if len(displayInfos) != 2 {
		s.Fatalf("Not enough connected displays: got %d; want 2", len(displayInfos))
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to setup ARC and Play Store: ", err)
	}
	defer d.Close(ctx)

	pkgs, err := a.InstalledPackages(ctx)
	if err != nil {
		s.Fatal("Failed to list the installed packages: ", err)
	}

	if _, ok := pkgs[gmailPackageName]; ok {
		s.Logf("%s is already installed", gmailPackageName)
	}

	s.Log("Installing ", gmailPackageName)
	installCtx, icancel := context.WithTimeout(ctx, 3*time.Minute)
	if err = playstore.InstallApp(installCtx, a, d, gmailPackageName, -1); err != nil {
		icancel()
		s.Fatalf("Failed to install %s: %v", gmailPackageName, err)
	}
	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Fatal("Failed to close Play Store: ", err)
	}

	recorder, err := cuj.NewRecorder(ctx, tconn, cuj.MetricConfigs()...)
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}

	setBatteryNormal, err := setup.SetBatteryDischarge(ctx, 20)
	if err != nil {
		s.Fatal("Failed to set battery discharge: ", err)
	} else {
		defer setBatteryNormal(ctx)
	}
	s.Log("Start to get browser start time")
	browserStartTime, err := cuj.GetOpenBrowserStartTime(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to get browser start time: ", err)
	}

	err = recorder.Run(ctx, func(ctx context.Context) error {
		ytConn, err := openYoutube(ctx, cr, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to open Youtube")
		}
		defer ytConn.Close()
		defer ytConn.CloseTarget(ctx)
		g, err := gmail.New(ctx, tconn, d)
		if err != nil {
			return err
		}

		receiver := s.RequiredVar("ui.cuj_username")
		subjectField := "CUJ Send Mail"
		contentField := "CUJ Testing Mail"
		if err := g.Send(ctx, d, receiver, subjectField, contentField); err != nil {
			return errors.Wrap(err, "failed to send email")
		}

		return nil
	})

	if err != nil {
		s.Error("Failed to run the scenario: ", err)
		for _, info := range displayInfos {
			path := fmt.Sprintf("%s/screenshot-multi-display-failed-test-%q.png", s.OutDir(), info.ID)
			if err := screenshot.CaptureChromeForDisplay(ctx, cr, info.ID, path); err != nil {
				s.Logf("Failed to capture screenshot for display ID %q: %v", info.ID, err)
			}
		}
	}

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "User.LoginTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(loginTime))
	pv.Set(perf.Metric{
		Name:      "Browser.StartTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(browserStartTime))

	if err = recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to report: ", err)
	}
	if err = pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store values: ", err)
	}
}

func openYoutube(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) (*chrome.Conn, error) {
	conn, err := cr.NewConn(ctx, yt4kVideoURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open Youtube in Chrome")
	}

	if err := webutil.WaitForQuiescence(ctx, conn, uiTimeout); err != nil {
		return nil, errors.Wrap(err, "failed to wait for youtube to finish loading")
	}

	if err := switchQualityTo4K(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to switch quality to 4K")
	}

	testing.Sleep(ctx, sleepTime)

	kw, err := input.Keyboard(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open the keyboard")
	}
	defer kw.Close()

	testing.ContextLog(ctx, "Enter fullscreen")
	if err := kw.Accel(ctx, "F"); err != nil {
		return nil, errors.Wrap(err, "failed to enter fullscreen")
	}

	testing.Sleep(ctx, sleepTime)

	testing.ContextLog(ctx, "Switch to the extended display")
	if err := kw.Accel(ctx, "Search+Alt+M"); err != nil {
		return nil, errors.Wrap(err, "failed to switch to the extended display")
	}

	testing.Sleep(ctx, sleepTime)
	return conn, nil
}

func switchQualityTo4K(ctx context.Context, tconn *chrome.TestConn) error {
	testing.ContextLog(ctx, "Click 'Settings'")
	if err := cuj.WaitAndClick(ctx, tconn, chromeui.FindParams{
		Role: chromeui.RoleTypePopUpButton,
		Name: "Settings"}, uiTimeout); err != nil {
		return errors.Wrap(err, "failed to click 'Settings'")
	}

	testing.ContextLog(ctx, "Click 'Quality'")
	if err := cuj.WaitAndClick(ctx, tconn, chromeui.FindParams{
		Attributes: map[string]interface{}{"name": regexp.MustCompile(`^Quality`)},
		Role:       chromeui.RoleTypeMenuItem}, uiTimeout); err != nil {
		return errors.Wrap(err, "failed to click 'Quality'")
	}

	testing.ContextLog(ctx, "Click '2160p'")
	if err := cuj.WaitAndClick(ctx, tconn, chromeui.FindParams{
		Attributes: map[string]interface{}{"name": regexp.MustCompile(`^2160p`)},
		Role:       chromeui.RoleTypeMenuItemRadio}, uiTimeout); err != nil {
		return errors.Wrap(err, "failed to click '2160p'")
	}

	return nil
}
