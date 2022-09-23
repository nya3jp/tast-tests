// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package clipboardhistory

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/clipboardhistory"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

type clipboardTest interface {
	openApp(ctx context.Context, f *clipboardhistory.FixtData) error
	closeApp(ctx context.Context) error
	pasteAndVerify(ctx context.Context, f *clipboardhistory.FixtData, text string) error
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
		Params: []testing.Param{{
			Name:    "ash_chrome",
			Fixture: "clipboardHistory",
			Val: &contextMenuClipboardTestParam{
				testName: "ash_chrome",
				testImpl: &browserTest{},
			},
		}, {
			Name:    "settings",
			Fixture: "clipboardHistory",
			Val: &contextMenuClipboardTestParam{
				testName: "settings",
				testImpl: &settingsTest{},
			},
		}, {
			Name:    "launcher",
			Fixture: "clipboardHistory",
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
	f := s.FixtValue().(*clipboardhistory.FixtData)

	clipboardItems := []string{"abc", "123"}
	for _, text := range clipboardItems {
		if err := ash.SetClipboard(ctx, f.TestConn, text); err != nil {
			s.Fatalf("Failed to set up %q into clipboard history: %v", text, err)
		}
	}

	param := s.Param().(*contextMenuClipboardTestParam)
	defer faillog.DumpUITreeWithScreenshotOnError(
		ctx, s.OutDir(), s.HasError, f.Chrome, fmt.Sprintf("%s_dump", param.testName))

	if err := param.testImpl.openApp(ctx, f); err != nil {
		s.Fatal("Failed to open app: ", err)
	}
	defer param.testImpl.closeApp(ctx)

	for _, text := range clipboardItems {
		if err := param.testImpl.pasteAndVerify(ctx, f, text); err != nil {
			s.Fatal("Failed to paste and verify: ", err)
		}
	}
}

// clearInputField clears the input field before pasting new contents.
func clearInputField(ctx context.Context, f *clipboardhistory.FixtData, inputFinder *nodewith.Finder) error {
	ui := f.UI
	kb := f.Keyboard

	if err := uiauto.Combine("clear input field",
		ui.LeftClick(inputFinder),
		ui.WaitUntilExists(inputFinder.Focused()),
		kb.AccelAction("Ctrl+A"),
		kb.AccelAction("Backspace"),
	)(ctx); err != nil {
		return err
	}

	nodeInfo, err := ui.Info(ctx, inputFinder)
	if err != nil {
		return errors.Wrap(err, "failed to get info for the input field")
	}

	if nodeInfo.Value != "" {
		return errors.Errorf("failed to clear value: %q", nodeInfo.Value)
	}

	return nil
}

// pasteAndVerify returns a function that pastes contents from clipboard and
// verifies the context menu behavior.
func pasteAndVerify(f *clipboardhistory.FixtData, inputFinder *nodewith.Finder, text string) uiauto.Action {
	return func(ctx context.Context) error {
		ui := f.UI

		testing.ContextLogf(ctx, "Paste %q", text)

		if err := clearInputField(ctx, f, inputFinder); err != nil {
			return errors.Wrap(err, "failed to clear input field before paste")
		}

		item := nodewith.Name(text).Role(role.MenuItem).HasClass("ClipboardHistoryTextItemView").First()
		if err := uiauto.Combine(fmt.Sprintf("paste %q from clipboard history", text),
			ui.RightClick(inputFinder),
			ui.DoDefault(nodewith.NameStartingWith("Clipboard").Role(role.MenuItem)),
			ui.WaitUntilGone(nodewith.HasClass("MenuItemView")),
			ui.LeftClick(item),
			ui.WaitForLocation(inputFinder),
		)(ctx); err != nil {
			return err
		}

		nodeInfo, err := ui.Info(ctx, inputFinder)
		if err != nil {
			return errors.Wrap(err, "failed to get info for the input field")
		}

		if !strings.Contains(nodeInfo.Value, text) {
			return errors.Wrapf(nil, "input field didn't contain the word: got %q; want %q", nodeInfo.Value, text)
		}

		return nil
	}
}

type browserTest struct {
	conn *chrome.Conn
}

func (b *browserTest) openApp(ctx context.Context, f *clipboardhistory.FixtData) error {
	conn, err := f.Browser.NewConn(ctx, "")
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

func (b *browserTest) pasteAndVerify(ctx context.Context, f *clipboardhistory.FixtData, text string) error {
	rootView := nodewith.NameStartingWith("about:blank").HasClass("BrowserRootView")
	searchbox := nodewith.Role(role.TextField).Name("Address and search bar").Ancestor(rootView)
	return pasteAndVerify(f, searchbox, text)(ctx)
}

type settingsTest struct {
	app *ossettings.OSSettings
}

func (s *settingsTest) openApp(ctx context.Context, f *clipboardhistory.FixtData) error {
	settings, err := ossettings.Launch(ctx, f.TestConn)
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

func (s *settingsTest) pasteAndVerify(ctx context.Context, f *clipboardhistory.FixtData, text string) error {
	return pasteAndVerify(f, ossettings.SearchBoxFinder, text)(ctx)
}

type launcherTest struct {
	tconn *chrome.TestConn
}

func (l *launcherTest) openApp(ctx context.Context, f *clipboardhistory.FixtData) error {
	l.tconn = f.TestConn
	return launcher.OpenBubbleLauncher(l.tconn)(ctx)
}

func (l *launcherTest) closeApp(ctx context.Context) error {
	return launcher.CloseBubbleLauncher(l.tconn)(ctx)
}

func (l *launcherTest) pasteAndVerify(ctx context.Context, f *clipboardhistory.FixtData, text string) error {
	search := nodewith.HasClass("SearchBoxView")
	searchbox := nodewith.HasClass("Textfield").Role(role.TextField).Ancestor(search)
	return pasteAndVerify(f, searchbox, text)(ctx)
}
