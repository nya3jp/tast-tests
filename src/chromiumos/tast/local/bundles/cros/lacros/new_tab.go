// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type newTabParam struct {
	bt   browser.Type
	opts []lacrosfixt.Option
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         NewTab,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests the various NewTab operations",
		Contacts:     []string{"neis@google.com", "lacros-team@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "ash",
			Val:  newTabParam{browser.TypeAsh, nil},
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               newTabParam{browser.TypeLacros, []lacrosfixt.Option{}},
		}, {
			Name:              "lacros_keep_alive",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               newTabParam{browser.TypeLacros, []lacrosfixt.Option{lacrosfixt.KeepAlive(true)}},
		}},
	})
}

// TODO(neis): This test is not using fixtures because there seems to be no
// good way to reset the browser state such that no tabs will get restored.

func NewTab(ctx context.Context, s *testing.State) {
	p := s.Param().(newTabParam)

	for _, param := range []struct {
		name             string
		openBrowser      func(ctx context.Context, cr *chrome.Chrome, bt browser.Type) (*browser.Browser, error)
		expectNewTabPage bool
	}{
		{
			name:             "shortcut",
			openBrowser:      openBrowserByNewTabShorcut,
			expectNewTabPage: true,
		},
		{
			name:             "shelf",
			openBrowser:      openBrowserByShelf,
			expectNewTabPage: false,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			cr, err := browserfixt.NewChrome(ctx, p.bt, lacrosfixt.NewConfig(p.opts...))
			if err != nil {
				s.Fatal("Failed to restart Chrome: ", err)
			}
			if err := prepareBrowser(ctx, cr, p.bt); err != nil {
				s.Fatal("Failed to prepare the browser: ", err)
			}
			br, err := param.openBrowser(ctx, cr, p.bt)
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}

			expectedTabs := []string{"chrome://version/"}
			if param.expectNewTabPage {
				expectedTabs = append(expectedTabs, chrome.NewTabURL)
			}
			if err := verifyTabs(ctx, br, expectedTabs); err != nil {
				s.Fatal("Failed to verify browser tabs: ", err)
			}
		})
	}
}

func openBrowserByShelf(ctx context.Context, cr *chrome.Chrome, bt browser.Type) (*browser.Browser, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Test API")
	}
	browserApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find browser app info")
	}
	if err := apps.Launch(ctx, tconn, browserApp.ID); err != nil {
		return nil, errors.Wrap(err, "failed to launch browser")
	}
	if err := ash.WaitForCondition(ctx, tconn, ash.BrowserTypeMatch(bt), nil); err != nil {
		return nil, errors.Wrap(err, "failed to find browser window")
	}
	br, err := browserfixt.Connect(ctx, cr, bt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to browser")
	}
	return br, nil
}

func openBrowserByNewTabShorcut(ctx context.Context, cr *chrome.Chrome, bt browser.Type) (*browser.Browser, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Test API")
	}
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()
	if err := kb.Accel(ctx, "Ctrl+t"); err != nil {
		return nil, errors.Wrap(err, "failed to send Ctrl+t")
	}
	if err := ash.WaitForCondition(ctx, tconn, ash.BrowserTypeMatch(bt), nil); err != nil {
		return nil, errors.Wrap(err, "failed to find browser window")
	}
	br, err := browserfixt.Connect(ctx, cr, bt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to browser")
	}
	return br, nil
}

func verifyTabs(ctx context.Context, br *browser.Browser, expectedURLs []string) error {
	tabs, err := br.Tabs(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get tabs")
	}
	actualURLs := make([]string, len(tabs))
	for i, tab := range tabs {
		actualURLs[i] = tab.URL
	}
	if !equal(actualURLs, expectedURLs) {
		return errors.Errorf("got %v, want %v", actualURLs, expectedURLs)
	}
	return nil
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func prepareTabs(ctx context.Context, cr *chrome.Chrome, bt browser.Type) error {
	conn, _, _, err := browserfixt.SetUpWithURL(ctx, cr, bt, "chrome://version/")
	if err != nil {
		return errors.Wrap(err, "failed to set up browser")
	}
	//defer closeBrowser(ctx) // XXX
	defer conn.Close()
	return nil
}

func prepareBrowser(ctx context.Context, cr *chrome.Chrome, bt browser.Type) error {
	if err := prepareTabs(ctx, cr, bt); err != nil {
		return errors.Wrap(err, "failed to prepare tabs")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to test API")
	}
	if err := ash.CloseAllWindows(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to close all windows")
	}
	// TODO(chromium:1316237): Remove sleep when this bug is fixed.
	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}
	return nil
}
