// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package browserfixt provides a function for obtaining a Browser instance for
// a given tast fixture and browser type.
package browserfixt

import (
	"context"

	"chromiumos/tast/errors"
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
// If browser type is TypeAsh, the fixture value must implement the HasChrome interface.
// If browser type is TypeLacros, the fixture value must be of lacrosfixt.FixtValue type.
// TODO(crbug.com/1315088): Replace f with just the HasChrome interface.
func SetUp(ctx context.Context, f interface{}, bt browser.Type) (*browser.Browser, func(ctx context.Context), error) {
	cr := f.(chrome.HasChrome).Chrome()
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

// SetUpWithURL is a combination of SetUp and NewConn that avoids an extra
// blank tab in the case of Lacros. The caller is responsible for closing the
// returned connection via its Close() method prior to calling the returned
// closure.
// TODO(crbug.com/1315088): Replace f with just the HasChrome interface.
func SetUpWithURL(ctx context.Context, f interface{}, bt browser.Type, url string) (*chrome.Conn, *browser.Browser, func(ctx context.Context), error) {
	cr := f.(chrome.HasChrome).Chrome()
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

// Expose various consts, func and types to tests that can access them importing browserfixt.

// LacrosDeployedBinary is lacrosfixt.LacrosDeployedBinary.
const LacrosDeployedBinary = lacrosfixt.LacrosDeployedBinary

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

// Connect connects to a running browser instance.
func Connect(ctx context.Context, cr *chrome.Chrome, bt browser.Type) (*browser.Browser, error) {
	switch bt {
	case browser.TypeAsh:
		return cr.Browser(), nil
	case browser.TypeLacros:
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to connect to ash-chrome test API")
		}
		l, err := lacros.Connect(ctx, tconn)
		if err != nil {
			return nil, errors.Wrap(err, "failed to connect to lacros-chrome")
		}
		return l.Browser(), nil
	default:
		return nil, errors.Errorf("unrecognized Chrome type %s", string(bt))
	}
}
