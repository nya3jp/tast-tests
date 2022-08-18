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
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// contextualMenusType represents the type of tests with different contextual menus.
type contextualMenusType int

// Enums of apps to be tested.
const (
	browserApp contextualMenusType = iota
	lacrosBrowserApp
	gmailApp
	settingsApp
	builtInLauncher
	filesApp
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ContextMenuClipboard,
		LacrosStatus: testing.LacrosVariantExists,
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
			Val:     browserApp,
		},
			// TODO(b/243339088): enable test on lacros browser once the issue is fixed
			// {
			// 	Name:    "browser_lacros",
			// 	Fixture: "lacros",
			// 	Val:     lacrosBrowserApp,
			// },
			{
				Name:    "gmail",
				Fixture: "chromeLoggedWithGaia",
				Val:     gmailApp,
			}, {
				Name:    "settings",
				Fixture: "chromeLoggedIn",
				Val:     settingsApp,
			}, {
				Name:    "launcher",
				Fixture: "chromeLoggedIn",
				Val:     builtInLauncher,
			},
			// TODO(crbug.com/1307522): enable test on filesapp once the issue is fixed
			// {
			// 	Name:    "filesapp",
			// 	Fixture: "chromeLoggedIn",
			// 	Val:     filesApp,
			// },
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

// ContextMenuClipboard verifies the clipboard option by left-clicking an option in Gmail, browser, launcher, Files app, and settings.
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

	browserType := browser.TypeAsh
	var test clipboardTest

	switch s.Param() {
	case browserApp:
		test = &browserTest{}
	case lacrosBrowserApp:
		test = &browserTest{}
		browserType = browser.TypeLacros
	case gmailApp:
		test = &gmailTest{}
	case settingsApp:
		test = &settingsTest{}
	case builtInLauncher:
		test = &launcherTest{}
	case filesApp:
		test = &filesappTest{}
	}

	cleanUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, browserType)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanUpCtx)

	resource := &clipboardResource{
		ui:           uiauto.New(tconn),
		kb:           kb,
		br:           br,
		tconn:        tconn,
		testContents: []string{"abc", "123"},
	}

	s.Log("Setup clipboard history")
	for _, text := range resource.testContents {
		s.Logf("Copy %q", text)

		if err := ash.SetClipboard(ctx, tconn, text); err != nil {
			s.Fatalf("Failed to set up %q into clipboard history: %v", text, err)
		}
	}

	if err := test.openApp(ctx, resource); err != nil {
		s.Fatal("Failed to open app: ", err)
	}
	defer test.closeApp(cleanUpCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanUpCtx, s.OutDir(), s.HasError, cr, fmt.Sprintf("%s_dump", s.TestName()))

	if err := test.pasteAndVerify(ctx, resource); err != nil {
		s.Fatal("Failed to paste and verify: ", err)
	}
}

// clearInputField clears the input field before pasting new contents.
func clearInputField(ctx context.Context, res *clipboardResource, inputFinder *nodewith.Finder) error {
	if err := uiauto.Combine("clear search box",
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

// pasteAndVerify pastes contents from clipboard and verifies the context menu behavior.
func pasteAndVerify(ctx context.Context, res *clipboardResource, inputFinder *nodewith.Finder, clearField bool) error {
	for _, text := range res.testContents {
		testing.ContextLogf(ctx, "Paste %q", text)

		if clearField {
			if err := clearInputField(ctx, res, inputFinder); err != nil {
				return errors.Wrap(err, "failed to clear input field before paste")
			}
		}

		item := nodewith.Name(text).Role(role.MenuItem).HasClass("ClipboardHistoryTextItemView").First()
		if err := uiauto.Combine(fmt.Sprintf("paste %q from clipboard history", text),
			res.ui.RightClick(inputFinder),
			res.ui.LeftClick(nodewith.NameStartingWith("Clipboard").Role(role.MenuItem)),
			res.ui.WaitUntilGone(nodewith.HasClass("MenuItemView")),
			res.ui.WaitUntilExists(item),
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
			return err
		}
		return b.conn.Close()
	}
	return nil
}

func (b *browserTest) pasteAndVerify(ctx context.Context, res *clipboardResource) error {
	rootView := nodewith.NameStartingWith("about:blank").HasClass("BrowserRootView")
	searchbox := nodewith.Role(role.TextField).Name("Address and search bar").Ancestor(rootView)
	return pasteAndVerify(ctx, res, searchbox, true /*clearField*/)
}

type gmailTest struct {
	conn *chrome.Conn
}

func (g *gmailTest) openApp(ctx context.Context, res *clipboardResource) error {
	conn, err := res.br.NewConn(ctx, "https://mail.google.com")
	if err != nil {
		return errors.Wrap(err, "failed to connect to chrome")
	}
	g.conn = conn
	return nil
}

func (g *gmailTest) closeApp(ctx context.Context) error {
	if g.conn != nil {
		if err := g.conn.CloseTarget(ctx); err != nil {
			return err
		}
		return g.conn.Close()
	}

	return nil
}

func (g *gmailTest) pasteAndVerify(ctx context.Context, res *clipboardResource) error {
	rootView := nodewith.Role(role.Window).NameContaining("Mail").HasClass("BrowserFrame")
	searchbox := nodewith.Role(role.TextField).Name("Search in mail").Ancestor(rootView)
	getStartedBtn := nodewith.Role(role.Button).Name("Get started").Ancestor(rootView)
	chatDialog := nodewith.Role(role.AlertDialog).NameContaining("Chat conversations").Ancestor(rootView)
	closeAlertBtn := nodewith.Role(role.Button).Name("Close").Ancestor(chatDialog)

	clearPrompts := uiauto.Combine("clear prompts",
		res.ui.LeftClick(getStartedBtn),
		res.ui.LeftClick(closeAlertBtn),
	)
	if err := uiauto.IfSuccessThen(res.ui.WaitUntilExists(getStartedBtn), clearPrompts)(ctx); err != nil {
		return errors.Wrap(err, "failed to clear prompts for gmail")
	}
	return pasteAndVerify(ctx, res, searchbox, true /*clearField*/)
}

type settingsTest struct {
	app *ossettings.OSSettings
}

func (set *settingsTest) openApp(ctx context.Context, res *clipboardResource) error {
	settings, err := ossettings.Launch(ctx, res.tconn)
	if err != nil {
		return errors.Wrap(err, "failed  to launch OS settings")
	}
	set.app = settings
	return nil
}

func (set *settingsTest) closeApp(ctx context.Context) error {
	if set.app != nil {
		return set.app.Close(ctx)
	}
	return nil
}

func (set *settingsTest) pasteAndVerify(ctx context.Context, res *clipboardResource) error {
	return pasteAndVerify(ctx, res, ossettings.SearchBoxFinder, true /*clearField*/)
}

type launcherTest struct {
	closeLauncher uiauto.Action
}

func (l *launcherTest) openApp(ctx context.Context, res *clipboardResource) error {
	l.closeLauncher = launcher.CloseBubbleLauncher(res.tconn)
	return launcher.OpenBubbleLauncher(res.tconn)(ctx)
}

func (l *launcherTest) closeApp(ctx context.Context) error {
	return l.closeLauncher(ctx)
}

func (l *launcherTest) pasteAndVerify(ctx context.Context, res *clipboardResource) error {
	search := nodewith.HasClass("SearchBoxView")
	searchbox := nodewith.HasClass("Textfield").Role(role.TextField).Ancestor(search)
	return pasteAndVerify(ctx, res, searchbox, true /*clearField*/)
}

type filesappTest struct {
	app *filesapp.FilesApp
}

func (f *filesappTest) openApp(ctx context.Context, res *clipboardResource) error {
	files, err := filesapp.Launch(ctx, res.tconn)
	if err != nil {
		return errors.Wrap(err, "failed to launch filesapp")
	}
	f.app = files
	return nil
}

func (f *filesappTest) closeApp(ctx context.Context) error {
	if f.app != nil {
		return f.app.Close(ctx)
	}
	return nil
}

func (f *filesappTest) pasteAndVerify(ctx context.Context, res *clipboardResource) error {
	filesappSearchbox := nodewith.Name("Search").Role(role.SearchBox).Onscreen()
	if err := uiauto.Combine("open search box",
		f.app.LeftClick(nodewith.Name("Search").Role(role.Button)),
		f.app.WaitUntilExists(filesappSearchbox),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to find search box")
	}

	// TODO(crbug.com/1307522): enable test on filesapp once the issue is fixed
	// Couldn't show clipboard option in context menu by right clicking.
	// Verify this part for filesapp after the issue is fixed.
	return pasteAndVerify(ctx, res, filesappSearchbox, false /*clearField*/)
}
