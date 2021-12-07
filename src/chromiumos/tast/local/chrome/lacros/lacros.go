// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacros implements a library used for utilities and communication with lacros-chrome on ChromeOS.
package lacros

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros/launcher"
	"chromiumos/tast/testing"
)

var pollOptions = &testing.PollOptions{Timeout: 10 * time.Second}

// Setup runs lacros-chrome if indicated by the given browser.Type and returns some objects and interfaces
// useful in tests. If the browser.Type is Lacros, it will return a non-nil LacrosChrome instance or an error.
// If the browser.Type is Ash it will return a nil LacrosChrome instance.
func Setup(ctx context.Context, f interface{}, bt browser.Type) (*chrome.Chrome, *launcher.LacrosChrome, ash.ConnSource, error) {
	if _, ok := f.(chrome.HasChrome); !ok {
		return nil, nil, nil, errors.Errorf("unrecognized FixtValue type: %v", f)
	}
	cr := f.(chrome.HasChrome).Chrome()

	switch bt {
	case browser.TypeAsh:
		return cr, nil, cr, nil
	case browser.TypeLacros:
		f := f.(launcher.FixtValue)
		l, err := launcher.LaunchLacrosChrome(ctx, f)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to launch lacros-chrome")
		}
		return cr, l, l, nil
	default:
		return nil, nil, nil, errors.Errorf("unrecognized Chrome type %s", string(bt))
	}
}

// CloseLacrosChrome closes the given lacros-chrome, if it is non-nil. Otherwise, it does nothing.
func CloseLacrosChrome(ctx context.Context, l *launcher.LacrosChrome) {
	if l != nil {
		l.Close(ctx) // Ignore error.
	}
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

// LaunchFromShelf launches lacros-chrome via shelf.
func LaunchFromShelf(ctx context.Context, tconn *chrome.TestConn, lacrosPath string) (*launcher.LacrosChrome, error) {
	const newTabTitle = "New Tab"

	// Make sure Lacros app is not running before launch.
	if ok, err := ash.AppRunning(ctx, tconn, apps.Lacros.ID); err != nil {
		return nil, errors.Wrap(err, "failed to check if app is not running before launch")
	} else if ok {
		return nil, errors.New("failed to launch lacros since app is already running. close before launch")
	}

	testing.ContextLog(ctx, "Launch lacros via Shelf")
	if err := ash.LaunchAppFromShelf(ctx, tconn, apps.Lacros.Name, apps.Lacros.ID); err != nil {
		return nil, errors.Wrap(err, "failed to launch lacros via shelf")
	}

	testing.ContextLog(ctx, "Wait for Lacros window")
	if err := launcher.WaitForLacrosWindow(ctx, tconn, newTabTitle); err != nil {
		return nil, errors.Wrap(err, "failed to wait for lacros")
	}

	l, err := launcher.ConnectToLacrosChrome(ctx, lacrosPath, launcher.LacrosUserDataDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to lacros")
	}
	return l, nil
}
