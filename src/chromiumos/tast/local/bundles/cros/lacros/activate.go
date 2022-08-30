// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"time"

	"github.com/google/go-cmp/cmp"

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
		Func:         Activate,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests browser activation via shelf and accelerator",
		Contacts:     []string{"neis@google.com", "lacros-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Params: []testing.Param{{
			// crbug.com/1332387
			//	Name: "ash",
			//	Val:  newTabParam{browser.TypeAsh, nil},
			//}, {
			Name: "no_keep_alive",
			Val:  newTabParam{browser.TypeLacros, []lacrosfixt.Option{}},
		}, {
			Name: "keep_alive",
			Val:  newTabParam{browser.TypeLacros, []lacrosfixt.Option{lacrosfixt.KeepAlive(true)}},
		}},
	})
}

// TODO(neis): This test is not using fixtures because there seems to be no
// good way to reset the browser state such that no tabs will get restored.

type appState string

const (
	appStateClosed     appState = "Closed"
	appStateBackground appState = "Background"
	appStateForeground appState = "Foreground"
)

func Activate(ctx context.Context, s *testing.State) {
	p := s.Param().(newTabParam)

	for _, param := range []struct {
		name                string
		browserPrecondition appState
		activateBrowser     func(ctx context.Context, tconn *chrome.TestConn) error
		expectNewTabPage    bool
	}{
		{
			name:                "closed_shortcut",
			browserPrecondition: appStateClosed,
			activateBrowser:     activateBrowserByNewTabShorcut,
			expectNewTabPage:    true,
		},
		{
			name:                "background_shortcut",
			browserPrecondition: appStateBackground,
			activateBrowser:     activateBrowserByNewTabShorcut,
			expectNewTabPage:    true,
		},
		{
			name:                "foreground_shortcut",
			browserPrecondition: appStateForeground,
			activateBrowser:     activateBrowserByNewTabShorcut,
			expectNewTabPage:    true,
		},
		{
			name:                "closed_shelf",
			browserPrecondition: appStateClosed,
			activateBrowser:     activateBrowserByShelf,
			expectNewTabPage:    false,
		},
		{
			name:                "background_shelf",
			browserPrecondition: appStateBackground,
			activateBrowser:     activateBrowserByShelf,
			expectNewTabPage:    true,
		},
		{
			name:                "foreground_shelf",
			browserPrecondition: appStateForeground,
			activateBrowser:     activateBrowserByShelf,
			expectNewTabPage:    true,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			cr, err := browserfixt.NewChrome(ctx, p.bt, lacrosfixt.NewConfig(p.opts...))
			if err != nil {
				s.Fatal("Failed to restart Chrome: ", err)
			}
			if err := prepareBrowser(ctx, cr, p.bt, param.browserPrecondition); err != nil {
				s.Fatal("Failed to prepare the browser: ", err)
			}

			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to connect to Test API: ", err)
			}
			if err := param.activateBrowser(ctx, tconn); err != nil {
				s.Fatal("Failed to activate the browser: ", err)
			}

			expectedTabs := []string{"chrome://version/"}
			if param.expectNewTabPage {
				expectedTabs = append(expectedTabs, chrome.NewTabURL)
			}
			if err := verifyTabs(ctx, cr, tconn, p.bt, expectedTabs); err != nil {
				s.Fatal("Failed to verify browser tabs: ", err)
			}
		})
	}
}

func activateBrowserByShelf(ctx context.Context, tconn *chrome.TestConn) error {
	browserApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find browser app info")
	}
	if err := apps.Launch(ctx, tconn, browserApp.ID); err != nil {
		return errors.Wrap(err, "failed to launch browser")
	}
	return nil
}

func activateBrowserByNewTabShorcut(ctx context.Context, _ *chrome.TestConn) error {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()
	if err := kb.Accel(ctx, "Ctrl+t"); err != nil {
		return errors.Wrap(err, "failed to send Ctrl+t")
	}
	return nil
}

func verifyTabs(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, bt browser.Type, expectedURLs []string) error {
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return ash.BrowserTypeMatch(bt)(w) && w.IsVisible && w.IsActive && w.HasFocus
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to find browser window")
	}
	br, brCleanUp, err := browserfixt.Connect(ctx, cr, bt)
	if err != nil {
		return errors.Wrap(err, "failed to connect to browser")
	}
	defer brCleanUp(ctx)
	tabs, err := br.CurrentTabs(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get tabs")
	}
	actualURLs := make([]string, len(tabs))
	for i, tab := range tabs {
		actualURLs[i] = tab.URL
	}
	if !cmp.Equal(actualURLs, expectedURLs) {
		return errors.Errorf("got %v, want %v", actualURLs, expectedURLs)
	}
	return nil
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

func prepareBrowser(ctx context.Context, cr *chrome.Chrome, bt browser.Type, browserPrecondition appState) error {
	if err := prepareTabs(ctx, cr, bt); err != nil {
		return errors.Wrap(err, "failed to prepare tabs")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to test API")
	}
	switch browserPrecondition {
	case appStateClosed:
		if err := ash.CloseAllWindows(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to close all windows")
		}
		// TODO(crbug.com/1316237): Remove sleep when this bug is fixed.
		if err := testing.Sleep(ctx, 3*time.Second); err != nil {
			return errors.Wrap(err, "failed to sleep")
		}
	case appStateBackground:
		window, err := ash.FindOnlyWindow(ctx, tconn, ash.BrowserTypeMatch(bt))
		if err != nil {
			return errors.Wrap(err, "failed to find browser window")
		}
		if err := ash.SetWindowStateAndWait(ctx, tconn, window.ID, ash.WindowStateMinimized); err != nil {
			return errors.Wrap(err, "failed to minimize browser window")
		}
	case appStateForeground:
		// Nothing to do.
	}
	return nil
}
