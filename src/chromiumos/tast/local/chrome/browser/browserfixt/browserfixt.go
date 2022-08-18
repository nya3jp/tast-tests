// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package browserfixt provides a function for obtaining a Browser instance for
// a given tast fixture and browser type.
package browserfixt

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfaillog"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

// SetUp returns a Browser instance for a given fixture value and browser type.
// It also returns a closure to be called in order to close the browser instance,
// after which the instance should not be used any further.
func SetUp(ctx context.Context, cr *chrome.Chrome, bt browser.Type) (*browser.Browser, func(ctx context.Context), error) {
	switch bt {
	case browser.TypeAsh:
		return cr.Browser(), func(context.Context) {}, nil
	case browser.TypeLacros:
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to connect to test API")
		}
		l, err := lacros.Launch(ctx, tconn)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to launch lacros-chrome")
		}
		closeLacros := func(ctx context.Context) {
			l.Close(ctx) // Ignore error.
		}
		return l.Browser(), closeLacros, nil
	default:
		return nil, nil, errors.Errorf("unrecognized browser type %s", string(bt))
	}
}

// SetUpIncognito returns a Browser instance in incognito mode for a given
// fixture value and browser type.
// It also returns a closure to be called in order to close the browser instance,
// after which the instance should not be used any further.
func SetUpIncognito(ctx context.Context, cr *chrome.Chrome, bt browser.Type) (*browser.Browser, func(ctx context.Context), error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to connect to test API")
	}
	if err := apps.LaunchChromeByShortcut(tconn, true)(ctx); err != nil {
		return nil, nil, errors.Wrap(err, "failed to launch incognito Chrome browser")
	}

	switch bt {
	case browser.TypeAsh:
		return cr.Browser(), func(context.Context) {}, nil
	case browser.TypeLacros:
		l, err := lacros.Connect(ctx, tconn)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to launch lacros-chrome")
		}
		closeLacros := func(ctx context.Context) {
			l.Close(ctx) // Ignore error.
		}
		return l.Browser(), closeLacros, nil
	default:
		return nil, nil, errors.Errorf("unrecognized browser type %s", string(bt))
	}
}

// SetUpWithURL is a combination of SetUp and NewConn that avoids an extra
// blank tab in the case of Lacros. The caller is responsible for closing the
// returned connection via its Close() method prior to calling the returned
// closure.
func SetUpWithURL(ctx context.Context, cr *chrome.Chrome, bt browser.Type, url string) (*chrome.Conn, *browser.Browser, func(ctx context.Context), error) {
	switch bt {
	case browser.TypeAsh:
		conn, err := cr.NewConn(ctx, url)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to connect to ash-chrome")
		}
		return conn, cr.Browser(), func(context.Context) {}, nil

	case browser.TypeLacros:
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to connect to test API")
		}

		l, err := lacros.LaunchWithURL(ctx, tconn, url)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to launch lacros-chrome")
		}
		conn, err := l.NewConnForTarget(ctx, chrome.MatchTargetURL(url))
		if err != nil {
			if err := l.Close(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to close lacros-chrome: ", err)
			}
			return nil, nil, nil, errors.Wrap(err, "failed to connect to lacros-chrome")
		}
		closeLacros := func(ctx context.Context) {
			if err := l.Close(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to close lacros-chrome: ", err)
			}
		}
		return conn, l.Browser(), closeLacros, nil

	default:
		return nil, nil, nil, errors.Errorf("unrecognized browser type %s", string(bt))
	}
}

// SetUpWithNewChrome returns a Browser instance along with a new Chrome instance created.
// This is useful when no fixture is used but the new chrome needs to be instantiated in test for a fresh UI restart between tests.
// It also returns a closure to be called in order to close the browser instance.
// The caller is responsible for calling the closure first, then Close() on the chrome instance for deferred cleanup.
// LacrosConfig is the configurations to be set to enable Lacros for use by tests.
// For convenience, DefaultLacrosConfig().WithVar(s) could be passed in when rootfs-lacros is needed as a primary browser unless specified with the runtime var.
func SetUpWithNewChrome(ctx context.Context, bt browser.Type, cfg *lacrosfixt.Config, opts ...chrome.Option) (*chrome.Chrome, *browser.Browser, func(ctx context.Context), error) {
	switch bt {
	case browser.TypeAsh:
		cr, err := chrome.New(ctx, opts...)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to connect to ash-chrome")
		}
		return cr, cr.Browser(), func(context.Context) {}, nil

	case browser.TypeLacros:
		lacrosOpts, err := cfg.Opts()
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to get default options")
		}
		opts = append(opts, lacrosOpts...)

		cr, err := chrome.New(ctx, opts...)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to connect to ash-chrome")
		}
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to connect to ash-chrome test API")
		}

		l, err := lacros.Launch(ctx, tconn)
		if err != nil {
			lacrosfaillog.Save(ctx, tconn)
			return nil, nil, nil, errors.Wrap(err, "failed to launch lacros-chrome")
		}
		closeBrowser := func(ctx context.Context) {
			if err := l.Close(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to close lacros-chrome: ", err)
			}
		}
		return cr, l.Browser(), closeBrowser, nil

	default:
		return nil, nil, nil, errors.Errorf("unrecognized browser type %s", string(bt))
	}
}

// NewChrome is basically SetUpWithNewChrome without the SetUp part.
// It restarts Chrome with, depending on the browser type, either just the
// given opts or the given opts plus those provided by the Lacros
// configuration. This is useful for situations where the browser will be
// launched via some UI interaction, for example.
func NewChrome(ctx context.Context, bt browser.Type, cfg *lacrosfixt.Config, opts ...chrome.Option) (*chrome.Chrome, error) {
	if bt == browser.TypeLacros {
		lacrosOpts, err := cfg.Opts()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get Lacros options")
		}
		opts = append(opts, lacrosOpts...)
	}
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to restart Chrome")
	}
	return cr, nil
}

// Connect connects to a running browser instance. It returns a closure for
// freeing resources when the connection is no longer needed (note that the
// closure does not close the browser).
func Connect(ctx context.Context, cr *chrome.Chrome, bt browser.Type) (*browser.Browser, func(ctx context.Context), error) {
	switch bt {
	case browser.TypeAsh:
		return cr.Browser(), func(context.Context) {}, nil
	case browser.TypeLacros:
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to connect to ash-chrome test API")
		}
		l, err := lacros.Connect(ctx, tconn)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to connect to lacros-chrome")
		}
		cleanUp := func(ctx context.Context) {
			l.CloseResources(ctx) // Ignore error.
		}
		return l.Browser(), cleanUp, nil
	default:
		return nil, nil, errors.Errorf("unrecognized Chrome type %s", string(bt))
	}
}
