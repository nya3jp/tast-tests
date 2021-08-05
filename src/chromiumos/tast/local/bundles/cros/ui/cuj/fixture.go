// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/local/logsaver"
	"chromiumos/tast/testing"
)

const resetTimeout = 30 * time.Second

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "loggedInToCUJUser",
		Desc:            "The main fixture used for UI CUJ tests",
		Contacts:        []string{"xiyuan@chromium.org"},
		Impl:            &loggedInToCUJUserFixture{},
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		Vars: []string{
			"ui.cuj_username",
			"ui.cuj_password",
			"cuj_username",
			"cuj_password",
		},
	})
	testing.AddFixture(&testing.Fixture{
		Name:            "loggedInAndKeepState",
		Desc:            "The CUJ test fixture which keeps login state",
		Contacts:        []string{"xiyuan@chromium.org"},
		Impl:            &loggedInToCUJUserFixture{keepState: true, webUITabStrip: true},
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		Vars: []string{
			"ui.cuj_username",
			"ui.cuj_password",
			"cuj_username",
			"cuj_password",
		},
	})
	testing.AddFixture(&testing.Fixture{
		Name:     "loggedInToCUJUserLacros",
		Desc:     "Fixture used for lacros variation of UI CUJ tests",
		Contacts: []string{"xiyuan@chromium.org"},
		Impl: launcher.NewStartedByData(launcher.PreExist, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.GAIALogin(getCreds(s)),
				chrome.ARCSupported(),
				chrome.ExtraArgs(arc.DisableSyncFlags()...),
				chrome.EnableFeatures("LacrosSupport"),
			}, nil
		}),
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
		Vars: []string{
			"ui.cuj_username",
			"ui.cuj_password",
			"cuj_username",
			"cuj_password",
			launcher.LacrosDeployedBinary,
		},
	})
	testing.AddFixture(&testing.Fixture{
		Name:            "loggedInToCUJUserLacrosWithARC",
		Desc:            "Fixture used for lacros variation of UI CUJ tests that also need ARC",
		Contacts:        []string{"xiyuan@chromium.org"},
		Impl:            &loggedInToCUJUserFixture{},
		Parent:          "loggedInToCUJUserLacros",
		SetUpTimeout:    chrome.GAIALoginTimeout + optin.OptinTimeout + arc.BootTimeout + 2*time.Minute,
		ResetTimeout:    resetTimeout,
		TearDownTimeout: resetTimeout,
	})
}

func getCreds(s *testing.FixtState) chrome.Creds {
	var username string
	var password string

	cujUser, userOk := s.Var("cuj_username")
	cujPass, passOk := s.Var("cuj_password")
	if userOk && passOk {
		username = cujUser
		password = cujPass
	} else {
		username = s.RequiredVar("ui.cuj_username")
		password = s.RequiredVar("ui.cuj_password")
	}

	return chrome.Creds{User: username, Pass: password}
}

func runningPackages(ctx context.Context, a *arc.ARC) (map[string]struct{}, error) {
	tasks, err := a.TaskInfosFromDumpsys(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "listing activities failed")
	}
	acts := make(map[string]struct{})
	for _, t := range tasks {
		for _, a := range t.ActivityInfos {
			acts[a.PackageName] = struct{}{}
		}
	}
	return acts, nil
}

// FixtureData is the struct returned by the preconditions.
type FixtureData struct {
	Chrome     *chrome.Chrome
	ARC        *arc.ARC
	LacrosFixt launcher.FixtData
}

type loggedInToCUJUserFixture struct {
	cr              *chrome.Chrome
	arc             *arc.ARC
	origRunningPkgs map[string]struct{}
	logMarker       *logsaver.Marker
	keepState       bool
	// webUITabStrip indicates whether we should run new chrome UI under tablet mode.
	// Remove this flag when new UI becomes default.
	webUITabStrip bool
	// Whether chrome is created by parent fixture.
	useParentChrome bool
}

func (f *loggedInToCUJUserFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	var cr *chrome.Chrome
	var lacrosFixt launcher.FixtData

	if s.ParentValue() != nil {
		lacrosFixt = s.ParentValue().(launcher.FixtData)
		cr = lacrosFixt.Chrome
		f.useParentChrome = true
	} else {
		func() {
			ctx, cancel := context.WithTimeout(ctx, chrome.LoginTimeout)
			defer cancel()

			opts := []chrome.Option{
				chrome.GAIALogin(getCreds(s)),
				chrome.ARCSupported(),
				chrome.ExtraArgs(arc.DisableSyncFlags()...),
			}
			if f.keepState {
				opts = append(opts, chrome.KeepState())
			}
			if f.webUITabStrip {
				opts = append(opts, chrome.EnableFeatures("WebUITabStrip"))
			}
			var err error
			cr, err = chrome.New(ctx, opts...)

			if err != nil {
				s.Fatal("Failed to start Chrome: ", err)
			}
			chrome.Lock()
		}()
		defer func() {
			if cr != nil {
				chrome.Unlock()
				if err := cr.Close(ctx); err != nil {
					s.Error("Failed to close Chrome: ", err)
				}
			}
		}()
	}

	enablePlayStore := true
	if f.keepState {
		// Check whether the play store has been enabled.
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to connect Test API: ", err)
		}
		st, err := arc.GetState(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get ARC state: ", err)
		}
		enablePlayStore = !st.Provisioned
	}

	if enablePlayStore {
		func() {
			const playStorePackageName = "com.android.vending"
			ctx, cancel := context.WithTimeout(ctx, optin.OptinTimeout+time.Minute)
			defer cancel()

			// Optin to Play Store.
			s.Log("Opting into Play Store")
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to get the test conn: ", err)
			}
			if err := optin.Perform(ctx, cr, tconn); err != nil {
				s.Fatal("Failed to optin to Play Store: ", err)
			}

			s.Log("Waiting for Playstore shown")
			if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
				return w.ARCPackageName == playStorePackageName
			}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
				// Playstore app window might not be shown, but optin should be successful
				// at this time. Log the error message but continue.
				s.Log("Failed to wait for the playstore window to be visible: ", err)
				return
			}

			if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
				s.Fatal("Failed to close Play Store: ", err)
			}
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if _, err := ash.GetARCAppWindowInfo(ctx, tconn, playStorePackageName); err == ash.ErrWindowNotFound {
					return nil
				} else if err != nil {
					return testing.PollBreak(err)
				}
				return errors.New("still seeing playstore window")
			}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
				s.Fatal("Failed to wait for the playstore window to be closed: ", err)
			}
		}()
	}

	var a *arc.ARC
	func() {
		ctx, cancel := context.WithTimeout(ctx, arc.BootTimeout)
		defer cancel()

		var err error
		if a, err = arc.New(ctx, s.OutDir()); err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}

		if f.origRunningPkgs, err = runningPackages(ctx, a); err != nil {
			if err := a.Close(ctx); err != nil {
				s.Error(ctx, "Failed to close ARC connection: ", err)
			}
			s.Fatal("Failed to list running packages: ", err)
		}
	}()
	f.cr = cr
	f.arc = a
	cr = nil
	return FixtureData{Chrome: f.cr, ARC: f.arc, LacrosFixt: lacrosFixt}
}

func (f *loggedInToCUJUserFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if !f.useParentChrome {
		chrome.Unlock()
	}

	if err := f.arc.Close(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to close ARC connection: ", err)
	}

	if !f.useParentChrome {
		if err := f.cr.Close(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to close Chrome connection: ", err)
		}
	}
}

func (f *loggedInToCUJUserFixture) Reset(ctx context.Context) error {
	// Stopping the running apps.
	running, err := runningPackages(ctx, f.arc)
	if err != nil {
		return errors.Wrap(err, "failed to get running packages")
	}
	for pkg := range running {
		if _, ok := f.origRunningPkgs[pkg]; ok {
			continue
		}
		testing.ContextLogf(ctx, "Stopping package %q", pkg)
		if err := f.arc.Command(ctx, "am", "force-stop", pkg).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to stop %q", pkg)
		}
	}

	if !f.useParentChrome {
		// Unlike ARC.preImpl, this does not uninstall apps. This is because we
		// typically want to reuse the same list of applications, and additional
		// installed apps wouldn't affect the test scenarios.
		if err = f.cr.ResetState(ctx); err != nil {
			return errors.Wrap(err, "failed to reset chrome")
		}
	}

	// Ensures that there are no toplevel windows left open.
	tconn, err := f.cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the test conn")
	}
	if all, err := ash.GetAllWindows(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to call ash.GetAllWindows")
	} else if len(all) != 0 {
		return errors.Wrapf(err, "toplevel window (%q) stayed open, total %d left", all[0].Name, len(all))
	}

	return nil
}

func (f *loggedInToCUJUserFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	if f.logMarker != nil {
		s.Log("A log marker is already created but not cleaned up")
	}
	logMarker, err := logsaver.NewMarker(f.cr.LogFilename())
	if err == nil {
		f.logMarker = logMarker
	} else {
		s.Log("Failed to start the log saver: ", err)
	}
}

func (f *loggedInToCUJUserFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if f.logMarker != nil {
		if err := f.logMarker.Save(filepath.Join(s.OutDir(), "chrome.log")); err != nil {
			s.Log("Failed to store per-test log data: ", err)
		}
		f.logMarker = nil
	}
}
