// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package clipboardhistory

import (
	"context"
	"fmt"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/clipboardhistory"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type clipboardResource struct {
	ui           *uiauto.Context
	kb           *input.KeyboardEventWriter
	br           *browser.Browser
	tconn        *chrome.TestConn
	testContents []string
}

type clipboardTest interface {
	openApp(ctx context.Context, res *clipboardResource) error
	closeApp(ctx context.Context) error
	pasteAndVerify(ctx context.Context, res *clipboardResource, text string) error
}

type contextMenuClipboardTestParam struct {
	testName string
	testImpl clipboardTest
}

func init() {
	testing.AddTest(&testing.Test{
		Func: ContextMenuClipboard,
		// TODO(b/243339088): There is no timeline for adding a clipboard option to
		// the Lacros address bar context menu. Therefore, no Lacros variant will
		// be added to this test for now.
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies the clipboard option in the context menu is working properly within several apps by left-clicking an option",
		Contacts: []string{
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"multipaste-eng@google.com",
			"victor.chen@cienet.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"ui.gaiaPoolDefault"},
		Fixture:      "chromeLoggedIn",
		Params: []testing.Param{{
			Name: "ash_chrome",
			Val: &contextMenuClipboardTestParam{
				testName: "ash_chrome",
				testImpl: &browserTest{},
			},
		}, {
			Name: "settings",
			Val: &contextMenuClipboardTestParam{
				testName: "settings",
				testImpl: &settingsTest{},
			},
		}, {
			Name: "launcher",
			Val: &contextMenuClipboardTestParam{
				testName: "launcher",
				testImpl: &launcherTest{},
			},
		},
		},
	})
}

// ContextMenuClipboard verifies that it is possible to open clipboard history
// via various surfaces' context menus.
func ContextMenuClipboard(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	res := &clipboardResource{
		ui:           uiauto.New(tconn),
		kb:           kb,
		br:           cr.Browser(),
		tconn:        tconn,
		testContents: []string{"abc", "123"},
	}

	clipboardItems := []string{"abc", "123"}
	for _, text := range clipboardItems {
		if err := ash.SetClipboard(ctx, res.tconn, text); err != nil {
			s.Fatalf("Failed to add %q to clipboard history: %v", text, err)
		}
	}

	param := s.Param().(*contextMenuClipboardTestParam)
	defer faillog.DumpUITreeWithScreenshotOnError(
		ctx, s.OutDir(), s.HasError, cr, fmt.Sprintf("%s_dump", param.testName))

	if err := param.testImpl.openApp(ctx, res); err != nil {
		s.Fatal("Failed to open app: ", err)
	}
	defer param.testImpl.closeApp(ctx)

	for _, text := range clipboardItems {
		if err := param.testImpl.pasteAndVerify(ctx, res, text); err != nil {
			s.Fatal("Failed to paste and verify: ", err)
		}
	}
}

func paste(ui *uiauto.Context, kb *input.KeyboardEventWriter, item *nodewith.Finder) uiauto.Action {
	return ui.LeftClick(item)
}

type browserTest struct {
	conn *chrome.Conn
}

func (b *browserTest) openApp(ctx context.Context, res *clipboardResource) error {
	conn, err := res.br.NewConn(ctx, "")
	if err != nil {
		return errors.Wrap(err, "failed to connect to chrome")
	}

	b.conn = conn
	return nil
}

func (b *browserTest) closeApp(ctx context.Context) error {
	if b.conn != nil {
		if err := b.conn.CloseTarget(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to close target: ", err)
		}
		if err := b.conn.Close(); err != nil {
			testing.ContextLog(ctx, "Failed to close connection: ", err)
		}
		b.conn = nil
	}
	return nil
}

func (b *browserTest) pasteAndVerify(ctx context.Context, res *clipboardResource, text string) error {
	rootView := nodewith.NameStartingWith("about:blank").HasClass("BrowserRootView")
	searchbox := nodewith.Role(role.TextField).Name("Address and search bar").Ancestor(rootView)
	return clipboardhistory.PasteAndVerify(res.ui, res.kb, searchbox /*contextMenu=*/, true, paste, text)(ctx)
}

type settingsTest struct {
	app *ossettings.OSSettings
}

func (s *settingsTest) openApp(ctx context.Context, res *clipboardResource) error {
	settings, err := ossettings.Launch(ctx, res.tconn)
	if err != nil {
		return errors.Wrap(err, "failed to launch OS settings")
	}

	s.app = settings
	return nil
}

func (s *settingsTest) closeApp(ctx context.Context) error {
	if s.app != nil {
		if err := s.app.Close(ctx); err != nil {
			return err
		}

		s.app = nil
	}
	return nil
}

func (s *settingsTest) pasteAndVerify(ctx context.Context, res *clipboardResource, text string) error {
	return clipboardhistory.PasteAndVerify(res.ui, res.kb, ossettings.SearchBoxFinder /*contextMenu=*/, true, paste, text)(ctx)
}

type launcherTest struct {
	tconn *chrome.TestConn
}

func (l *launcherTest) openApp(ctx context.Context, res *clipboardResource) error {
	l.tconn = res.tconn
	return launcher.OpenBubbleLauncher(l.tconn)(ctx)
}

func (l *launcherTest) closeApp(ctx context.Context) error {
	return launcher.CloseBubbleLauncher(l.tconn)(ctx)
}

func (l *launcherTest) pasteAndVerify(ctx context.Context, res *clipboardResource, text string) error {
	search := nodewith.HasClass("SearchBoxView")
	searchbox := nodewith.HasClass("Textfield").Role(role.TextField).Ancestor(search)
	return clipboardhistory.PasteAndVerify(res.ui, res.kb, searchbox /*contextMenu=*/, true, paste, text)(ctx)
}
