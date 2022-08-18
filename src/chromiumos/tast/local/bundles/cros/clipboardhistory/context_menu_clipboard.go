// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package clipboardhistory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type contextMenuClipboardTestParam struct {
	testName string
	testImpl clipboardTest
}

func init() {
	testing.AddTest(&testing.Test{
		Func: ContextMenuClipboard,
		// TODO(b/243339088): There is no timeline for adding a clipboard option to the Lacros context menu,
		// therefore, lacros will not be added to this test for now.
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies the clipboard option in the context menu is working properly within several apps by left-clicking an option",
		Contacts: []string{
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"victor.chen@cienet.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"ui.gaiaPoolDefault"},
		Params: []testing.Param{{
			Name:    "browser",
			Fixture: "chromeLoggedIn",
			Val: &contextMenuClipboardTestParam{
				testName: "ash_Chrome",
				testImpl: &browserTest{},
			},
		}, {
			Name:    "gmail",
			Fixture: "chromeLoggedInWithGaia",
			Val: &contextMenuClipboardTestParam{
				testName: "gmail_App",
				testImpl: &gmailTest{},
			},
		}, {
			Name:    "settings",
			Fixture: "chromeLoggedIn",
			Val: &contextMenuClipboardTestParam{
				testName: "settings_App",
				testImpl: &settingsTest{},
			},
		}, {
			Name:    "launcher",
			Fixture: "chromeLoggedIn",
			Val: &contextMenuClipboardTestParam{
				testName: "bubble_launcher",
				testImpl: &launcherTest{},
			},
		},
		},
	})
}

type clipboardResource struct {
	ui           *uiauto.Context
	kb           *input.KeyboardEventWriter
	br           *browser.Browser
	tconn        *chrome.TestConn
	testContents []string
}

// ContextMenuClipboard verifies that it is possible to open clipboard history via various surfaces' context menus.
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

	cleanUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	resource := &clipboardResource{
		ui:           uiauto.New(tconn),
		kb:           kb,
		br:           cr.Browser(),
		tconn:        tconn,
		testContents: []string{"abc", "123"},
	}

	s.Log("Setup clipboard history")
	for _, text := range resource.testContents {
		if err := ash.SetClipboard(ctx, tconn, text); err != nil {
			s.Fatalf("Failed to set up %q into clipboard history: %v", text, err)
		}
	}

	param := s.Param().(*contextMenuClipboardTestParam)

	if err := param.testImpl.openApp(ctx, resource); err != nil {
		s.Fatal("Failed to open app: ", err)
	}
	defer param.testImpl.closeApp(cleanUpCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanUpCtx, s.OutDir(), s.HasError, cr, fmt.Sprintf("%s_dump", param.testName))

	if err := param.testImpl.pasteAndVerify(ctx, resource); err != nil {
		s.Fatal("Failed to paste and verify: ", err)
	}
}

// clearInputField clears the input field before pasting new contents.
func clearInputField(ctx context.Context, res *clipboardResource, inputFinder *nodewith.Finder) error {
	if err := uiauto.Combine("clear input field",
		res.ui.LeftClick(inputFinder),
		res.ui.WaitUntilExists(inputFinder.Focused()),
		res.kb.AccelAction("Ctrl+A"),
		res.kb.AccelAction("Backspace"),
	)(ctx); err != nil {
		return err
	}

	nodeInfo, err := res.ui.Info(ctx, inputFinder)
	if err != nil {
		return errors.Wrap(err, "failed to get info for the input field")
	}
	if nodeInfo.Value != "" {
		return errors.Errorf("failed to clear value: %q", nodeInfo.Value)
	}
	return nil
}

// pasteAndVerify returns a function that pastes contents from clipboard and verifies the context menu behavior.
func pasteAndVerify(res *clipboardResource, inputFinder *nodewith.Finder) uiauto.Action {
	return func(ctx context.Context) error {
		for _, text := range res.testContents {
			testing.ContextLogf(ctx, "Paste %q", text)

			if err := clearInputField(ctx, res, inputFinder); err != nil {
				return errors.Wrap(err, "failed to clear input field before paste")
			}

			item := nodewith.Name(text).Role(role.MenuItem).HasClass("ClipboardHistoryTextItemView").First()
			if err := uiauto.Combine(fmt.Sprintf("paste %q from clipboard history", text),
				res.ui.RightClick(inputFinder),
				res.ui.DoDefault(nodewith.NameStartingWith("Clipboard").Role(role.MenuItem)),
				res.ui.WaitUntilGone(nodewith.HasClass("MenuItemView")),
				res.ui.LeftClick(item),
				res.ui.WaitForLocation(inputFinder),
			)(ctx); err != nil {
				return err
			}

			nodeInfo, err := res.ui.Info(ctx, inputFinder)
			if err != nil {
				return errors.Wrap(err, "failed to get info for the input field")
			}
			if !strings.Contains(nodeInfo.Value, text) {
				return errors.Wrapf(nil, "input field didn't contain the word: got %q; want %q", nodeInfo.Value, text)
			}
		}
		return nil
	}
}

type clipboardTest interface {
	openApp(ctx context.Context, res *clipboardResource) error
	closeApp(ctx context.Context) error
	pasteAndVerify(ctx context.Context, res *clipboardResource) error
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

func (b *browserTest) pasteAndVerify(ctx context.Context, res *clipboardResource) error {
	rootView := nodewith.NameStartingWith("about:blank").HasClass("BrowserRootView")
	searchbox := nodewith.Role(role.TextField).Name("Address and search bar").Ancestor(rootView)
	return pasteAndVerify(res, searchbox)(ctx)
}

type gmailTest struct {
	conn *chrome.Conn
}

func (g *gmailTest) openApp(ctx context.Context, res *clipboardResource) error {
	conn, err := res.br.NewConn(ctx, "https://mail.google.com")
	if err != nil {
		return errors.Wrap(err, "failed to open Gmail")
	}
	g.conn = conn
	return nil
}

func (g *gmailTest) closeApp(ctx context.Context) error {
	if g.conn != nil {
		if err := g.conn.CloseTarget(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to close target: ", err)
		}
		if err := g.conn.Close(); err != nil {
			testing.ContextLog(ctx, "Failed to close connection: ", err)
		}
		g.conn = nil
	}

	return nil
}

func (g *gmailTest) pasteAndVerify(ctx context.Context, res *clipboardResource) error {
	rootView := nodewith.Role(role.Window).NameContaining("Mail").HasClass("BrowserFrame")
	searchbox := nodewith.Role(role.TextField).Name("Search in mail").Ancestor(rootView)
	getStartedBtn := nodewith.Role(role.Button).Name("Get started").Ancestor(rootView)
	chatDialog := nodewith.Role(role.AlertDialog).NameContaining("Chat conversations").Ancestor(rootView)
	closeAlertBtn := nodewith.Role(role.Button).Name("Close").Ancestor(chatDialog)
	clearPrompts := uiauto.IfSuccessThen(
		res.ui.Exists(getStartedBtn),
		uiauto.Combine("clear prompts",
			res.ui.LeftClick(getStartedBtn),
			res.ui.LeftClick(closeAlertBtn),
		),
	)

	// Retry a few times in case the prompts pop up in the middle of actions.
	return uiauto.Retry(3,
		uiauto.Combine("clear Gmail prompts and then paste-verify",
			clearPrompts,
			pasteAndVerify(res, searchbox),
		),
	)(ctx)
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

func (s *settingsTest) pasteAndVerify(ctx context.Context, res *clipboardResource) error {
	return pasteAndVerify(res, ossettings.SearchBoxFinder)(ctx)
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

func (l *launcherTest) pasteAndVerify(ctx context.Context, res *clipboardResource) error {
	search := nodewith.HasClass("SearchBoxView")
	searchbox := nodewith.HasClass("Textfield").Role(role.TextField).Ancestor(search)
	return pasteAndVerify(res, searchbox)(ctx)
}
