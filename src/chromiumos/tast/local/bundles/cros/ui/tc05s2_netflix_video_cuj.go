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
	"chromiumos/tast/local/bundles/cros/ui/netflix"
	"chromiumos/tast/local/bundles/cros/ui/videocuj"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TC05S2NetflixVideoCUJ,
		Desc:         "Measures the smoothess of switch between full screen Netflix video and a tab",
		Contacts:     []string{"xiyuan@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      5 * time.Minute,
		Pre:          cuj.LoginKeepState(),
		Vars: []string{
			"ui.cuj_username",
			"ui.cuj_password",
			"ui.netflix_emailid",
			"ui.netflix_password",
		},
	})
}

func TC05S2NetflixVideoCUJ(ctx context.Context, s *testing.State) {
	tabletMode := false

	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	cr := s.PreValue().(cuj.PreKeepData).Chrome
	a := s.PreValue().(cuj.PreKeepData).ARC
	loginTime := s.PreValue().(cuj.PreKeepData).LoginTime
	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to setup ARC and Play Store: ", err)
	}
	defer d.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	tabChecker, err := cuj.NewTabCrashChecker(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create TabCrashChecker: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	pc := pointer.NewMouseController(tconn)
	defer pc.Close()

	recorder, err := cuj.NewRecorder(ctx, tconn, cuj.MetricConfigs()...)
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}
	defer recorder.Close(closeCtx)

	const (
		gmailPkg   = "com.google.android.gm"
		youtubePkg = "com.google.android.youtube"
	)

	s.Log("Check installed packages")
	pkgs, err := a.InstalledPackages(ctx)
	if err != nil {
		s.Fatal("Failed to list the installed packages: ", err)
	}

	for _, pkgName := range []string{gmailPkg, youtubePkg} {
		if _, ok := pkgs[pkgName]; ok {
			s.Logf("%s is already installed", pkgName)
			continue
		}
		s.Log("Installing ", pkgName)
		installCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		if err = playstore.InstallApp(installCtx, a, d, pkgName, -1); err != nil {
			cancel()
			s.Fatalf("Failed to install %s: %v", pkgName, err)
		}
		if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
			s.Fatal("Failed to close Play Store: ", err)
		}
		cancel()
	}

	s.Log("Open Gmail app")
	if _, err := gmail.New(ctx, tconn, d); err != nil {
		s.Fatal("Failed to open Gmail: ", err)
	}

	var gmWinID int
	if all, err := ash.GetAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to get all window: ", err)
	} else if len(all) != 1 {
		s.Fatalf("Expect 1 window, got %d", len(all))
	} else {
		gmWinID = all[0].ID
	}

	s.Log("Start to get browser start time")
	browserStartTime, err := cuj.GetOpenBrowserStartTime(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to get browser start time: ", err)
	}

	s.Log("Sign in netflix")
	n, err := netflix.New(ctx, s, tconn, cr)
	defer n.SignOut(ctx)

	s.Log("Go to watch netflix video")
	n.Play(ctx, "https://www.netflix.com/watch/80026431")

	var nfWinID int
	if all, err := ash.GetAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to get all window: ", err)
	} else if len(all) != 2 {
		s.Fatalf("Expect 2 windows, got %d", len(all))
	} else {
		if gmWinID == all[0].ID {
			nfWinID = all[1].ID
		} else {
			nfWinID = all[0].ID
		}
	}

	if err := n.WaitForLoading(ctx, time.Second*30); err != nil {
		s.Fatal("Failed to wait for netflix to finish loading: ", err)
	}

	if err := ash.HideVisibleNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to hide ash notification: ", err)
	}

	// Hold alt a bit then tab to show the window cycle list.
	altTab := func() error {
		if err := kb.AccelPress(ctx, "Alt"); err != nil {
			return errors.Wrap(err, "failed to press alt")
		}
		defer kb.AccelRelease(ctx, "Alt")
		if err := testing.Sleep(ctx, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to wait")
		}
		if err := kb.Accel(ctx, "Tab"); err != nil {
			return errors.Wrap(err, "failed to type tab")
		}
		if err := testing.Sleep(ctx, time.Second); err != nil {
			return errors.Wrap(err, "failed to wait")
		}
		return nil
	}

	enterFullscreen := func() error {
		nfWin, err := ash.GetWindow(ctx, tconn, nfWinID)
		if err != nil {
			return errors.Wrap(err, "failed to get netflix window")
		} else if nfWin.State == ash.WindowStateFullscreen {
			return errors.New("alreay in fullscreen")
		}

		testing.ContextLog(ctx, "before switching to fullscreen, nfWin.State = ", nfWin.State)

		if err := kb.Accel(ctx, "F"); err != nil {
			testing.ContextLog(ctx, "kb.Accel(ctx, 'F') return failure : ", err)
			return err
		}

		testing.ContextLog(ctx, "after switching to fullscreen, nfWin.State = ", nfWin.State)

		if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
			return w.ID == nfWinID && w.State == ash.WindowStateFullscreen
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to wait for fullscreen")
		}

		return nil
	}

	testing.Sleep(ctx, time.Second*5)
	cuj.WaitAndClick(ctx, tconn, ui.FindParams{Name: "Block", Role: ui.RoleTypeButton}, time.Second)

	setBatteryNormal, err := setup.SetBatteryDischarge(ctx, 20)
	if err != nil {
		s.Fatal("Failed to set battery discharge: ", err)
	} else {
		defer setBatteryNormal(ctx)
	}

	if err = recorder.Run(ctx, func(ctx context.Context) error {
		s.Log("Make video fullscreen")
		if err := enterFullscreen(); err != nil {
			return errors.Wrap(err, "failed to enter fullscreen")
		}

		testing.Sleep(ctx, time.Second*5)

		s.Log("Switch away from fullscreen video")
		if err := altTab(); err != nil {
			return errors.Wrap(err, "failed to alt-tab")
		}

		if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
			return w.ID == nfWinID && !w.IsActive && !w.IsAnimating
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to wait netflix window deactivate")
		}

		s.Log("Switch back to fullscreen video")
		if err := altTab(); err != nil {
			return errors.Wrap(err, "failed to alt-tab")
		}

		if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
			return w.ID == nfWinID && w.IsActive && w.State == ash.WindowStateFullscreen
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to wait active fullscreen netflix window")
		}

		if err := videocuj.DoYoutubeCUJ(ctx, tconn, a, d); err != nil {
			return errors.Wrap(err, "failed to do YouTube cuj")
		}

		return nil
	}); err != nil {
		s.Fatal("Failed: ", err)
	}

	s.Log("Press Enter")
	if err := kb.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Failed to type enter: ", err)
	}
	// Before recording the metrics, check if there is any tab crashed.
	if err := tabChecker.Check(ctx); err != nil {
		s.Fatal("Tab renderer crashed: ", err)
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
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
