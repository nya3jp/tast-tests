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
	"chromiumos/tast/local/chrome/lacros/launcher"
	"chromiumos/tast/local/policyutil/fixtures"
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
func Setup(ctx context.Context, f interface{}, crt ChromeType) (*chrome.Chrome, *launcher.LacrosChrome, ash.ConnSource, error) {
	cr, err := GetChrome(ctx, f)
	testing.ContextLogf(ctx, "Entered lacros.Setup with fixture (%v, %T) and chrome is: %v", f, f, cr)
	if err != nil {
		return nil, nil, nil, err
	}

	switch crt {
	case ChromeTypeChromeOS:
		return cr, nil, cr, nil
	case ChromeTypeLacros:
		f := f.(launcher.FixtData)
		l, err := launcher.LaunchLacrosChrome(ctx, f)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to launch lacros-chrome")
		}
		return cr, l, l, nil
	default:
		return nil, nil, nil, errors.Errorf("unrecognized Chrome type %s", string(crt))
	}
}

// GetChrome gets the *chrome.Chrome object given some FixtData, which may be lacros launcher.FixtData.
func GetChrome(ctx context.Context, f interface{}) (*chrome.Chrome, error) {
	switch f.(type) {
	case *chrome.Chrome:
		return f.(*chrome.Chrome), nil
	case *launcher.FixtData:
		return f.(*launcher.FixtData).Chrome, nil
	case launcher.FixtData:
		return f.(launcher.FixtData).Chrome, nil
	case *fixtures.FixtData:
		return f.(*fixtures.FixtData).Chrome, nil
	default:
		return nil, errors.Errorf("unrecognized FixtValue type: %v and type %T", f, f)
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

// ShelfLaunch launches lacros-chrome via shelf.
func ShelfLaunch(ctx context.Context, tconn *chrome.TestConn, f launcher.FixtData) (*launcher.LacrosChrome, error) {
	const newTabTitle = "New Tab"

	// Ensure shelf is visible in case of tablet mode.
	if err := ash.ShowHotseat(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to show hot seat")
	}

	testing.ContextLog(ctx, "Launch lacros via Shelf")
	if err := ash.LaunchAppFromShelf(ctx, tconn, apps.Lacros.Name, apps.Lacros.ID); err != nil {
		return nil, errors.Wrap(err, "failed to launch lacros via shelf")
	}

	testing.ContextLog(ctx, "Wait for Lacros window")
	if err := launcher.WaitForLacrosWindow(ctx, tconn, newTabTitle); err != nil {
		return nil, errors.Wrap(err, "failed to wait for lacros")
	}

	l, err := launcher.ConnectToLacrosChrome(ctx, f.LacrosPath, launcher.LacrosUserDataDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to lacros")
	}
	return l, nil
}
