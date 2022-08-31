// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SmartSelectionChrome,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test ARC's smart selections show up in Chrome's right click menu",
		Contacts:     []string{"djacobo@chromium.org", "jorgegil@google.com", "chromeos-sw-engprod@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		VarDeps:      []string{"arc.SmartSelectionChrome.username", "arc.SmartSelectionChrome.password"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Val:               browser.TypeAsh,
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val:               browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"android_p", "lacros"},
			Val:               browser.TypeLacros,
		}, {
			Name:              "lacros_vm",
			ExtraSoftwareDeps: []string{"android_vm", "lacros"},
			Val:               browser.TypeLacros,
		}},
	})
}

func SmartSelectionChrome(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	username := s.RequiredVar("arc.SmartSelectionChrome.username")
	password := s.RequiredVar("arc.SmartSelectionChrome.password")

	opts := []chrome.Option{chrome.GAIALogin(chrome.Creds{User: username, Pass: password}), chrome.ARCSupported()}
	if s.Param().(browser.Type) == browser.TypeLacros {
		var err error
		opts, err = lacrosfixt.NewConfig(lacrosfixt.ChromeOptions(opts...)).Opts()
		if err != nil {
			s.Fatal("Failed to compute chrome options: ", err)
		}
	}
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC by user policy: ", err)
	}
	defer a.Close(ctx)
	if err := a.WaitIntentHelper(ctx); err != nil {
		s.Fatal("Failed to wait for ARC Intent Helper: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	// Open page with an address on it.
	conn, err := br.NewConn(ctx, "https://google.com/search?q=1600+amphitheatre+parkway")
	if err != nil {
		s.Fatal("Failed to create new Chrome connection: ", err)
	}
	defer conn.Close()

	// Wait for the address to appear.
	address := nodewith.Name("1600 amphitheatre parkway").Role(role.StaticText).First()
	if err := ui.WaitUntilExists(address)(ctx); err != nil {
		s.Fatal("Failed to wait for address to load: ", err)
	}

	// Select the address and setup watcher to wait for text selection event
	if err := ui.WaitForEvent(nodewith.Root(),
		event.TextSelectionChanged,
		ui.Select(address, 0, address, 25))(ctx); err != nil {
		s.Fatal("Failed to select address: ", err)
	}

	// Right click the selected address and ensure the smart selection map option is available.
	if err := waitForMapOption(ctx, ui, address); err != nil {
		s.Log("Failed to show map option: ", err)
		// After timeout, dump all the menuItems if possible, this should provide a clear
		// idea whether items are missing in the menu or the menu not being there at all.
		menu := nodewith.ClassName("MenuItemView")
		menuItems, err := ui.NodesInfo(ctx, menu)
		if err != nil {
			s.Fatal("Could not find context menu items: ", err)
		}
		var items []string
		for _, item := range menuItems {
			items = append(items, item.Name)
		}
		s.Fatalf("Found %d menu items, including: %s", len(items), strings.Join(items, " / "))
	}
}

func waitForMapOption(ctx context.Context, ui *uiauto.Context, address *nodewith.Finder) error {
	mapOption := nodewith.Name("Map").Role(role.MenuItem)
	testing.ContextLog(ctx, "Polling until Map option is found")
	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := ui.RightClick(address)(ctx); err != nil {
			return errors.Wrap(err, "failed to right click on address")
		}
		if err := ui.WithTimeout(3 * time.Second).WithInterval(100 * time.Millisecond).WaitUntilExists(mapOption)(ctx); err != nil {
			// Did not find Map, click again to close the context menu.
			testing.ContextLog(ctx, "Did not find Map in context menu")
			if e := ui.RightClick(address)(ctx); e != nil {
				return testing.PollBreak(errors.Wrap(e, "failed to close context menu"))
			}
			return errors.Wrap(err, "Map option not found")
		}
		testing.ContextLog(ctx, "Map option found")
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second, Interval: 500 * time.Millisecond})
}
