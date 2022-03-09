// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package browserfixt provides a function for obtaining a Browser instance for
// a given tast fixture and browser type.
package browserfixt

import (
	"context"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

// SetUp returns a Browser instance for a given fixture value and browser type.
// It also returns a closure to be called in order to close the browser instance,
// after which the instance should not be used any further.
// If browser type is TypeAsh, the fixture value must implement the HasChrome interface.
// If browser type is TypeLacros, the fixture value must be of lacrosfixt.FixtValue type.
func SetUp(ctx context.Context, f interface{}, bt browser.Type) (*browser.Browser, func(ctx context.Context), error) {
	switch bt {
	case browser.TypeAsh:
		cr := f.(chrome.HasChrome).Chrome()
		return cr.Browser(), func(context.Context) {}, nil
	case browser.TypeLacros:
		f := f.(lacrosfixt.FixtValue)
		l, err := lacros.Launch(ctx, f)
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
func SetUpWithURL(ctx context.Context, f interface{}, bt browser.Type, url string) (*chrome.Conn, *browser.Browser, func(ctx context.Context), error) {
	switch bt {
	case browser.TypeAsh:
		cr := f.(chrome.HasChrome).Chrome()
		conn, err := cr.NewConn(ctx, url)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to connect to ash-chrome")
		}
		return conn, cr.Browser(), func(context.Context) {}, nil

	case browser.TypeLacros:
		f := f.(lacrosfixt.FixtValue)
		l, err := lacros.LaunchWithURL(ctx, f, url)
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

// LacrosDeployedBinary is lacrosfixt.LacrosDeployedBinary.
const LacrosDeployedBinary = lacrosfixt.LacrosDeployedBinary

// LacrosConfig is lacrosfixt.LacrosConfig.
type LacrosConfig = lacrosfixt.LacrosConfig

// DefaultLacrosConfig defaults to using rootfs-lacros as a primary browser.
// To support the deployed lacros-chrome from the command line, call WithVar(s) on this with the var lacrosDeployedBinary specified.
var DefaultLacrosConfig = LacrosConfig{
	Mode:         lacrosfixt.Rootfs,
	Availability: lacrosfixt.LacrosPrimary,
}

// SetUpWithNewChrome returns a Browser instance along with a new Chrome instance created.
// This is useful when no fixture is used but the new chrome needs to be instantiated in test for a fresh UI restart between tests.
// It also returns a closure to be called in order to close the browser instance.
// The caller is responsible for calling the closure and Close() on the chrome instance for deferred cleanup.
// s is TestingState that can accept *testing.State to read the vars declared in test for setting up Lacros.
func SetUpWithNewChrome(ctx context.Context, bt browser.Type, cfg LacrosConfig, opts ...chrome.Option) (*chrome.Chrome, *browser.Browser, func(ctx context.Context), error) {
	switch bt {
	case browser.TypeAsh:
		cr, err := chrome.New(ctx, opts...)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to connect to ash-chrome")
		}
		return cr, cr.Browser(), func(context.Context) {}, nil

	case browser.TypeLacros:
		defaultOpts, err := lacrosfixt.DefaultOpts(cfg)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to get default options")
		}
		opts = append(opts, defaultOpts...)

		cr, err := chrome.New(ctx, opts...)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to connect to ash-chrome")
		}
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to connect to ash-chrome test API")
		}

		lacrosPath, err := lacrosfixt.EnsureLacrosReadyForLaunch(ctx, cfg)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to wait for lacros-chrome to be ready")
		}

		l, err := lacros.LaunchFromShelf(ctx, tconn, lacrosPath)
		if err != nil {
			// TODO(crbug.com/1298962): Replace with lacrosfaillog to save lacros.log on failure for debugging.
			if out, ok := testing.ContextOutDir(ctx); !ok {
				testing.ContextLog(ctx, "OutDir not found")
			} else if err := fsutil.CopyFile(filepath.Join(lacros.UserDataDir, "lacros.log"), filepath.Join(out, "lacros.log")); err != nil {
				testing.ContextLogf(ctx, "Failed to copy lacros.log from %v to %v, err: %v", lacros.UserDataDir, out, err)
			}
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
