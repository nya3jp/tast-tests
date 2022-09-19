// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/uiauto/clipboardhistory"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

type clipboardTest interface {
	openApp(ctx context.Context, env *clipboardhistory.TestEnv) error
	closeApp(ctx context.Context) error
	pasteAndVerify(ctx context.Context, env *clipboardhistory.TestEnv, clipboardItems []string) error
}

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
			"multipaste@google.com",
			"victor.chen@cienet.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"ui.gaiaPoolDefault"},
		Params: []testing.Param{{
			Name:    "ash_chrome",
			Fixture: "chromeLoggedIn",
			Val: &contextMenuClipboardTestParam{
				testName: "ash_chrome",
				testImpl: &browserTest{},
			},
		}, {
			Name:    "gmail",
			Fixture: "chromeLoggedInWithGaia",
			Val: &contextMenuClipboardTestParam{
				testName: "gmail",
				testImpl: &gmailTest{},
			},
		}, {
			Name:    "settings",
			Fixture: "chromeLoggedIn",
			Val: &contextMenuClipboardTestParam{
				testName: "settings",
				testImpl: &settingsTest{},
			},
		}, {
			Name:    "launcher",
			Fixture: "chromeLoggedIn",
			Val: &contextMenuClipboardTestParam{
				testName: "launcher",
				testImpl: &launcherTest{},
			},
		},
		},
	})
}

// ContextMenuClipboard verifies that it is possible to open clipboard history via various surfaces' context menus.
func ContextMenuClipboard(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	env, err := clipboardhistory.SetUpEnv(ctx, cr, browser.TypeAsh)
	if err != nil {
		s.Fatal("Failed to set up test environment: ", err)
	}
	defer env.Kb.Close()
	defer env.Cb(ctx)

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, env.Tconn)
	defer faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)

	cleanUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	s.Log("Setup clipboard history")
	clipboardItems := []string{"abc", "123"}
	for _, text := range clipboardItems {
		if err := ash.SetClipboard(ctx, env.Tconn, text); err != nil {
			s.Fatalf("Failed to set up %q into clipboard history: %v", text, err)
		}
	}

	param := s.Param().(*contextMenuClipboardTestParam)

	if err := param.testImpl.openApp(ctx, &env); err != nil {
		s.Fatal("Failed to open app: ", err)
	}
	defer param.testImpl.closeApp(cleanUpCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanUpCtx, s.OutDir(), s.HasError, cr, fmt.Sprintf("%s_dump", param.testName))

	if err := param.testImpl.pasteAndVerify(ctx, &env, clipboardItems); err != nil {
		s.Fatal("Failed to paste and verify: ", err)
	}
}

// clearInputField clears the input field before pasting new contents.
func clearInputField(ctx context.Context, env *clipboardhistory.TestEnv, inputFinder *nodewith.Finder) error {
	if err := uiauto.Combine("clear input field",
		env.UI.LeftClick(inputFinder),
		env.UI.WaitUntilExists(inputFinder.Focused()),
		env.Kb.AccelAction("Ctrl+A"),
		env.Kb.AccelAction("Backspace"),
	)(ctx); err != nil {
		return err
	}

	nodeInfo, err := env.UI.Info(ctx, inputFinder)
	if err != nil {
		return errors.Wrap(err, "failed to get info for the input field")
	}
	if nodeInfo.Value != "" {
		return errors.Errorf("failed to clear value: %q", nodeInfo.Value)
	}
	return nil
}

// pasteAndVerify returns a function that pastes contents from clipboard and verifies the context menu behavior.
func pasteAndVerify(env *clipboardhistory.TestEnv, inputFinder *nodewith.Finder, clipboardItems []string) uiauto.Action {
	return func(ctx context.Context) error {
		for _, text := range clipboardItems {
			testing.ContextLogf(ctx, "Paste %q", text)

			if err := clearInputField(ctx, env, inputFinder); err != nil {
				return errors.Wrap(err, "failed to clear input field before paste")
			}

			item := nodewith.Name(text).Role(role.MenuItem).HasClass("ClipboardHistoryTextItemView").First()
			if err := uiauto.Combine(fmt.Sprintf("paste %q from clipboard history", text),
				env.UI.RightClick(inputFinder),
				env.UI.DoDefault(nodewith.NameStartingWith("Clipboard").Role(role.MenuItem)),
				env.UI.WaitUntilGone(nodewith.HasClass("MenuItemView")),
				env.UI.LeftClick(item),
				env.UI.WaitForLocation(inputFinder),
			)(ctx); err != nil {
				return err
			}

			nodeInfo, err := env.UI.Info(ctx, inputFinder)
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

type browserTest struct {
	conn *chrome.Conn
}

func (b *browserTest) openApp(ctx context.Context, env *clipboardhistory.TestEnv) error {
	conn, err := env.Br.NewConn(ctx, "")
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

func (b *browserTest) pasteAndVerify(ctx context.Context, env *clipboardhistory.TestEnv, clipboardItems []string) error {
	rootView := nodewith.NameStartingWith("about:blank").HasClass("BrowserRootView")
	searchbox := nodewith.Role(role.TextField).Name("Address and search bar").Ancestor(rootView)
	return pasteAndVerify(env, searchbox, clipboardItems)(ctx)
}

type gmailTest struct {
	conn *chrome.Conn
}

func (g *gmailTest) openApp(ctx context.Context, env *clipboardhistory.TestEnv) error {
	conn, err := env.Br.NewConn(ctx, "https://mail.google.com")
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

func (g *gmailTest) pasteAndVerify(ctx context.Context, env *clipboardhistory.TestEnv, clipboardItems []string) error {
	rootView := nodewith.Role(role.Window).NameContaining("Mail").HasClass("BrowserFrame")
	searchbox := nodewith.Role(role.TextField).Name("Search in mail").Ancestor(rootView)
	getStartedBtn := nodewith.Role(role.Button).Name("Get started").Ancestor(rootView)
	chatDialog := nodewith.Role(role.AlertDialog).NameContaining("Chat conversations").Ancestor(rootView)
	closeAlertBtn := nodewith.Role(role.Button).Name("Close").Ancestor(chatDialog)
	clearPrompts := uiauto.IfSuccessThen(
		env.UI.Exists(getStartedBtn),
		uiauto.Combine("clear prompts",
			env.UI.LeftClick(getStartedBtn),
			env.UI.LeftClick(closeAlertBtn),
		),
	)

	// Retry a few times in case the prompts pop up in the middle of actions.
	return uiauto.Retry(3,
		uiauto.Combine("clear Gmail prompts and then paste-verify",
			clearPrompts,
			pasteAndVerify(env, searchbox, clipboardItems),
		),
	)(ctx)
}

type settingsTest struct {
	app *ossettings.OSSettings
}

func (s *settingsTest) openApp(ctx context.Context, env *clipboardhistory.TestEnv) error {
	settings, err := ossettings.Launch(ctx, env.Tconn)
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

func (s *settingsTest) pasteAndVerify(ctx context.Context, env *clipboardhistory.TestEnv, clipboardItems []string) error {
	return pasteAndVerify(env, ossettings.SearchBoxFinder, clipboardItems)(ctx)
}

type launcherTest struct {
	tconn *chrome.TestConn
}

func (l *launcherTest) openApp(ctx context.Context, env *clipboardhistory.TestEnv) error {
	l.tconn = env.Tconn
	return launcher.OpenBubbleLauncher(l.tconn)(ctx)
}

func (l *launcherTest) closeApp(ctx context.Context) error {
	return launcher.CloseBubbleLauncher(l.tconn)(ctx)
}

func (l *launcherTest) pasteAndVerify(ctx context.Context, env *clipboardhistory.TestEnv, clipboardItems []string) error {
	search := nodewith.HasClass("SearchBoxView")
	searchbox := nodewith.HasClass("Textfield").Role(role.TextField).Ancestor(search)
	return pasteAndVerify(env, searchbox, clipboardItems)(ctx)
}
