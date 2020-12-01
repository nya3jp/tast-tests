// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacros implements a library used for utilities and communication with lacros-chrome on ChromeOS.
package lacros

import (
	"context"
	"strings"
	"time"

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
)

// ChromeType indicates which type of Chrome browser to be used.
type ChromeType string

const (
	// ChromeTypeChromeOS indicates we are using the ChromeOS system's Chrome browser.
	ChromeTypeChromeOS ChromeType = "chromeos"
	// ChromeTypeLacros indicates we are using lacros-chrome.
	ChromeTypeLacros ChromeType = "lacros"
)

var pollOptions = &testing.PollOptions{Timeout: 10 * time.Second}

// Setup runs lacros-chrome if indicated by the given ChromeType and returns some objects and interfaces
// useful in tests. If the ChromeType is ChromeTypeLacros, it will return a non-nil LacrosChrome instance or an error.
// If the ChromeType is ChromeTypeChromeOS it will return a nil LacrosChrome instance.
func Setup(ctx context.Context, pd interface{}, crt ChromeType) (*chrome.Chrome, *launcher.LacrosChrome, ash.ConnSource, error) {
	switch crt {
	case ChromeTypeChromeOS:
		return pd.(*chrome.Chrome), nil, pd.(*chrome.Chrome), nil
	case ChromeTypeLacros:
		pd := pd.(launcher.PreData)
		l, err := launcher.LaunchLacrosChrome(ctx, pd)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to launch lacros-chrome")
		}
		return pd.Chrome, l, l, nil
	default:
		return nil, nil, nil, errors.Errorf("unrecognized Chrome type %s", string(crt))
	}
}

// GetChrome gets the *chrome.Chrome object given some PreData, which may be lacros launcher.PreData.
func GetChrome(ctx context.Context, pd interface{}) (*chrome.Chrome, error) {
	switch pd.(type) {
	case *chrome.Chrome:
		return pd.(*chrome.Chrome), nil
	case launcher.PreData:
		return pd.(launcher.PreData).Chrome, nil
	default:
		return nil, errors.New("unrecognized PreData type. Couldn't find a chrome object")
	}
}

// CloseLacrosChrome closes the given lacros-chrome, if it is non-nil. Otherwise, it does nothing.
func CloseLacrosChrome(ctx context.Context, l *launcher.LacrosChrome) {
	if l != nil {
		l.Close(ctx) // Ignore error.
	}
}

// CloseAboutBlank finds all targets that are about:blank, closes them, then waits until they are gone.
// windowsExpectedClosed indicates how many windows that we expect to be closed from doing this operation.
func CloseAboutBlank(ctx context.Context, tconn *chrome.TestConn, ds *cdputil.Session, windowsExpectedClosed int) error {
	prevWindows, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return err
	}

	targets, err := ds.FindTargets(ctx, chrome.MatchTargetURL(chrome.BlankURL))
	if err != nil {
		return errors.Wrap(err, "failed to query for about:blank pages")
	}
	allPages, err := ds.FindTargets(ctx, func(t *target.Info) bool { return t.Type == "page" })
	if err != nil {
		return errors.Wrap(err, "failed to query for all pages")
	}

	for _, info := range targets {
		if err := ds.CloseTarget(ctx, info.TargetID); err != nil {
			return err
		}
	}
	return testing.Poll(ctx, func(ctx context.Context) error {
		// If we are closing all lacros targets, then lacros Chrome will exit. In that case, we won't be able to
		// communicate with it, so skip checking the targets. Since closing all lacros targets will close all
		// lacros windows, the window check below is necessary and sufficient.
		if len(targets) != len(allPages) {
			targets, err := ds.FindTargets(ctx, chrome.MatchTargetURL(chrome.BlankURL))
			if err != nil {
				return testing.PollBreak(err)
			}
			if len(targets) != 0 {
				return errors.New("not all about:blank targets were closed")
			}
		}

		windows, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return testing.PollBreak(err)
		}
		if len(prevWindows)-len(windows) != windowsExpectedClosed {
			return errors.Errorf("expected %d windows to be closed, got %d closed",
				windowsExpectedClosed, len(prevWindows)-len(windows))
		}

		return nil
	}, &testing.PollOptions{Timeout: time.Minute})
}

func waitForWindowWithPredicate(ctx context.Context, ctconn *chrome.TestConn, p func(*ash.Window) bool) (*ash.Window, error) {
	if err := ash.WaitForCondition(ctx, ctconn, p, pollOptions); err != nil {
		return nil, err
	}
	return ash.FindWindow(ctx, ctconn, p)
}

// FindFirstBlankWindow finds the first window whose title is 'about:blank'.
func FindFirstBlankWindow(ctx context.Context, ctconn *chrome.TestConn) (*ash.Window, error) {
	return waitForWindowWithPredicate(ctx, ctconn, func(w *ash.Window) bool {
		return strings.Contains(w.Title, "about:blank")
	})
}

// FindFirstNonBlankWindow finds the first window whose title is not 'about:blank'.
func FindFirstNonBlankWindow(ctx context.Context, ctconn *chrome.TestConn) (*ash.Window, error) {
	return waitForWindowWithPredicate(ctx, ctconn, func(w *ash.Window) bool {
		return !strings.Contains(w.Title, "about:blank")
	})
}
