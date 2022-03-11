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
	"chromiumos/tast/local/chrome/ash"
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

// CreateWindows is a util that makes the transition from ash.CreateWindows easy for both ash-chrome and lacros-chrome.
// It is necessary to address the difference of application life cycle between them.
// Unlike ash-chrome which instance is always available even with no window open, lacros-chrome needs to opens at least one extra blank window before instantiating.
// CreateWindows create n browser windows with specified URL and wait for them to become visible.
// It will fail and return an error if at least one request fails to fulfill. Note that this will
// parallelize the requests to create windows, which may be bad if the caller
// wants to measure the performance of Chrome. This should be used for a
// preparation, before the measurement happens.
// tconn is from *ash-chrome*.
func CreateWindows(ctx context.Context, f interface{}, bt browser.Type, tconn *chrome.TestConn, url string, n int) (*browser.Browser, func(ctx context.Context), error) {
	// TODO(crbug.com/1290318): Find a way to open a lacros-chrome with any chrome:// URLs.
	// Due to crbug.com/1290318 lacros-chrome can't open any chrome:// URLs other than blank page upon initializing.
	// For blank URL, use SetUpWithURL for the first window and ash.CreateWindows for the rest.
	// For other URLs to work around, let lacros-chrome start with a blank page, open the given number of windows with the given URL, then close the first blank page.
	switch url {
	case "", chrome.BlankURL:
		// Open the first window with a given URL on startup.
		_, br, closeBrowser, err := SetUpWithURL(ctx, f, bt, url)
		if err != nil {
			return nil, nil, err
		}
		// Open one less windows.
		if n > 1 {
			if err := ash.CreateWindows(ctx, tconn, br, url, n-1); err != nil {
				return nil, nil, err
			}
		}
		return br, closeBrowser, nil

	default:
		// Open a blank window for lacros-chrome, but no window for ash-chrome.
		// XXX: This looks ugly to copy the entire SetUp func, modify it to return the lacros instance and eventually call CloseAboutBlank.
		// This could be simplied with "br, closeBrowser, err := SetUp(ctx, f, bt)"
		// and https://crrev.com/c/3426334 (util to close any browser window with the URL)
		br, closeBrowser, l, err := func(ctx context.Context, f interface{}, bt browser.Type) (*browser.Browser, func(ctx context.Context), *lacros.Lacros, error) {
			switch bt {
			case browser.TypeAsh:
				cr := f.(chrome.HasChrome).Chrome()
				return cr.Browser(), func(context.Context) {}, nil, nil
			case browser.TypeLacros:
				f := f.(lacrosfixt.FixtValue)
				l, err := lacros.Launch(ctx, f)
				if err != nil {
					return nil, nil, nil, errors.Wrap(err, "failed to launch lacros-chrome")
				}
				closeLacros := func(ctx context.Context) {
					l.Close(ctx) // Ignore error.
				}
				return l.Browser(), closeLacros, l, nil
			default:
				return nil, nil, nil, errors.Errorf("unrecognized browser type %s", string(bt))
			}
		}(ctx, f, bt)
		if err != nil {
			return nil, nil, err
		}
		// Open the exact number of windows.
		if err := ash.CreateWindows(ctx, tconn, br, url, n); err != nil {
			return nil, nil, err
		}
		// Close the extra blank window if lacros-chrome.
		if bt == browser.TypeLacros {
			if err := l.CloseAboutBlank(ctx, tconn, 1); err != nil {
				return nil, nil, errors.Wrap(err, "failed to close about:blank for lacros")
			}
		}
		return br, closeBrowser, nil
	}
}
