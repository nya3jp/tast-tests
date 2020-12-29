// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/gmail"
	"chromiumos/tast/local/bundles/cros/ui/meetcuj"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/remote/chameleon"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const gmailPackageName = "com.google.android.gm"

func init() {
	testing.AddTest(&testing.Test{
		Func:         TC07S2ExtendedDisplayVideoConferenceCUJ,
		Desc:         "Measures the smoothness of external display CUJ",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"android_p", "chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      15 * time.Minute,
		Vars: []string{
			"ui.cuj_username",
			"ui.cuj_password",
			"chameleon",
		},
		Pre: cuj.LoginKeepState(),
	})
}

func TC07S2ExtendedDisplayVideoConferenceCUJ(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(cuj.PreKeepData).Chrome
	a := s.PreValue().(cuj.PreKeepData).ARC
	loginTime := s.PreValue().(cuj.PreKeepData).LoginTime
	chameleonAddr := s.RequiredVar("chameleon")
	tabletMode := false

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

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

	if len(displayInfos) < 2 {
		s.Fatal("Not enough display, at least 2 displays, need one more external")
	}

	var externalDisplayID string
	for _, info := range displayInfos {
		if !info.IsInternal {
			externalDisplayID = info.ID
		}
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
	defer recorder.Close(ctx)

	dsTracker := perfutil.NewDisplaySmoothnessTracker()
	if err := dsTracker.Start(ctx, tconn, externalDisplayID); err != nil {
		s.Fatal("Failed to start display smoothness tracking: ", err)
	}

	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, scancel := ctxutil.Shorten(ctx, 2*time.Second)
	defer scancel()
	defer dsTracker.Close(closeCtx, tconn)

	back, err := setup.SetBatteryDischarge(ctx, 20)
	if err != nil {
		s.Fatal("Failed to set battery discharge: ", err)
	} else {
		defer back(ctx)
	}

	browserStartTime, err := cuj.GetOpenBrowserStartTime(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to get browser start time: ", err)
	}

	err = recorder.Run(ctx, func(ctx context.Context) error {
		confConn, err := meetcuj.JoinGoogleMeetConferenceWithEnterprise(ctx, cr)
		if err != nil {
			return err
		}
		defer confConn.Close()
		defer confConn.CloseTarget(ctx)

		// Switch conference room page to external display
		if err := kb.Accel(ctx, "Search+Alt+M"); err != nil {
			return err
		}
		testing.Sleep(ctx, time.Second)
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
	defer cuj.CloseAllWindows(ctx, tconn)

	if err != nil {
		s.Fatal("Failed to run recorder: ", err)
	}

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "Browser.StartTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(browserStartTime))
	pv.Set(perf.Metric{
		Name:      "User.LoginTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(loginTime))

	if err = recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to report: ", err)
	}
	if err = pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store values: ", err)
	}
}
