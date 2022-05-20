// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/internal/cdputil"
	"chromiumos/tast/local/chrome/internal/driver"
	"chromiumos/tast/local/chrome/jslog"
	"chromiumos/tast/testing"
)

// Setup runs lacros-chrome if indicated by the given browser.Type and returns some objects and interfaces
// useful in tests. If the browser.Type is Lacros, it will return a non-nil Lacros instance or an error.
// If the browser.Type is Ash it will return a nil Lacros instance.
// TODO(crbug.com/1315088): Replace f with just the HasChrome interface.
func Setup(ctx context.Context, f interface{}, bt browser.Type) (*chrome.Chrome, *Lacros, ash.ConnSource, error) {
	if _, ok := f.(chrome.HasChrome); !ok {
		return nil, nil, nil, errors.Errorf("unrecognized FixtValue type: %v", f)
	}
	cr := f.(chrome.HasChrome).Chrome()

	switch bt {
	case browser.TypeAsh:
		return cr, nil, cr, nil
	case browser.TypeLacros:
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to get TestConn")
		}
		l, err := Launch(ctx, tconn)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to launch lacros-chrome")
		}
		return cr, l, l, nil
	default:
		return nil, nil, nil, errors.Errorf("unrecognized Chrome type %s", string(bt))
	}
}

// Connect connects to a running lacros instance (e.g launched by the UI) and returns a Lacros object that can be used to interact with it.
func Connect(ctx context.Context, tconn *chrome.TestConn) (l *Lacros, retErr error) {
	debuggingPortPath := filepath.Join(UserDataDir, "DevToolsActivePort")

	info, err := InfoSnapshot(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get lacros info")
	}
	execPath := filepath.Join(info.LacrosPath, "chrome")

	agg := jslog.NewAggregator()
	defer func() {
		if retErr != nil {
			agg.Close()
		}
	}()

	sess, err := driver.NewSession(ctx, execPath, debuggingPortPath, cdputil.WaitPort, agg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to debugging port")
	}
	return &Lacros{
		agg:    agg,
		sess:   sess,
		ctconn: tconn,
	}, nil
}

// Launch launches lacros.
func Launch(ctx context.Context, tconn *chrome.TestConn) (*Lacros, error) {
	// Make sure Lacros app is not running before launch.
	if running, err := ash.AppRunning(ctx, tconn, apps.Lacros.ID); err != nil {
		return nil, errors.Wrap(err, "failed to check if app is not running before launch")
	} else if running {
		return nil, errors.New("failed to launch lacros since app is already running. close before launch")
	}

	testing.ContextLog(ctx, "Waiting for lacros to be launched")
	if err := apps.Launch(ctx, tconn, apps.Lacros.ID); err != nil {
		return nil, errors.Wrap(err, "failed to launch lacros")
	}
	// Poll to make sure lacros actually runs. This is specially important to avoid the case where
	// lacros binary is still loading when launch is triggered and it's assumed that launching
	// lacros on login is disabled (--disable-login-lacros-opening).
	// TODO(crbug.com/1316237): This may be unnecessary if BrowserManager wouldn't drop requests.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		info, err := InfoSnapshot(ctx, tconn)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get lacros info"))
		}

		if !info.Running {
			if err := apps.Launch(ctx, tconn, apps.Lacros.ID); err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to launch lacros"))
			}
			return errors.Wrap(err, "re-trying lacros launch")
		}
		return nil
	}, &testing.PollOptions{Timeout: 120 * time.Second, Interval: 1 * time.Second}); err != nil {
		return nil, errors.Wrap(err, "failed to run lacros")
	}

	testing.ContextLog(ctx, "Wait for Lacros window")
	if err := WaitForLacrosWindow(ctx, tconn, ""); err != nil {
		return nil, errors.Wrap(err, "failed to wait for lacros")
	}

	l, err := Connect(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to lacros")
	}
	return l, nil
}

// LaunchWithURL launches lacros-chrome and ensures there is one page open
// with the given URL. Note that this function expects lacros to be closed
// as a precondition.
func LaunchWithURL(ctx context.Context, tconn *chrome.TestConn, url string) (*Lacros, error) {
	l, err := Launch(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch Lacros")
	}

	// Get all pages.
	ts, err := l.FindTargets(ctx, chrome.MatchAllPages())
	if err != nil {
		return nil, errors.Wrap(err, "failed to find pages")
	}

	if len(ts) != 1 {
		return nil, errors.Wrapf(err, "expected only one page target, got %v", ts)
	}

	conn, err := l.NewConnForTarget(ctx, chrome.MatchTargetID(ts[0].TargetID))
	if err := conn.Navigate(ctx, url); err != nil {
		return nil, errors.Wrap(err, "failed to navigate to url")
	}

	return l, nil
}
