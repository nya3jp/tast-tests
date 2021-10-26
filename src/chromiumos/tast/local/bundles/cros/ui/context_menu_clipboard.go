// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
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

func init() {
	testing.AddTest(&testing.Test{
		Func:         ContextMenuClipboard,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Verifies the clipboard option in context menu is working properly within several apps",
		Contacts: []string{
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"ting.chen@cienet.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"ui.gaiaPoolDefault"},
		Fixture:      "chromeLoggedWithGaia",
	})
}

type clipboardResource struct {
	ui           *uiauto.Context
	kb           *input.KeyboardEventWriter
	br           *browser.Browser
	tconn        *chrome.TestConn
	testContents []string
}

// ContextMenuClipboard verifies the clipboard option in Gmail, browser, launcher, Files app, settings.
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

	resource := &clipboardResource{
		ui:           uiauto.New(tconn),
		kb:           kb,
		br:           cr.Browser(),
		tconn:        tconn,
		testContents: []string{"abc", "123"},
	}

	s.Log("Setup clipboard history")
	if err := setUpHistory(ctx, resource, cr, s.OutDir()); err != nil {
		s.Fatal("Failed to set up clipbaord history: ", err)
	}

	tests := map[string]clipboardTest{
		"browser":  &webpage{},
		"gmail":    &gmail{},
		"settings": &setting{},
		"launcher": &launch{},
		// TODO(crbug.com/1307522): enable test on filesapp once the issue is fixed
		// "filesApp": &fileapp{},
	}
	for name, test := range tests {
		f := func(ctx context.Context, s *testing.State) {
			cleanUpCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
			defer cancel()

			if err := test.openApp(ctx, resource); err != nil {
				s.Fatal("Failed to open app: ", err)
			}
			defer test.closeApp(cleanUpCtx)
			defer faillog.DumpUITreeWithScreenshotOnError(cleanUpCtx, s.OutDir(), s.HasError, cr, fmt.Sprintf("%s_dump", name))

			if err := test.pasteAndVerify(ctx, resource); err != nil {
				s.Fatal("Failed to paste and verify: ", err)
			}
		}

		if !s.Run(ctx, name, f) {
			s.Errorf("Failed to run subtest %q", name)
		}
	}
}

// setUpHistory copies the test contents to clipboard for further tests.
func setUpHistory(ctx context.Context, res *clipboardResource, cr *chrome.Chrome, outdir string) (retErr error) {
	conn, err := res.br.NewConn(ctx, "")
	if err != nil {
		return errors.Wrap(err, "failed to connect to chrome")
	}
	defer func(ctx context.Context) {
		faillog.DumpUITreeWithScreenshotOnError(ctx, outdir, func() bool { return retErr != nil }, cr, "ui_setHistory")
		conn.CloseTarget(ctx)
		conn.Close()
	}(ctx)

	rootView := nodewith.NameStartingWith("about:blank").HasClass("BrowserRootView")
	addressBarNode := nodewith.Role(role.TextField).Name("Address and search bar").Ancestor(rootView)
	for _, text := range res.testContents {
		testing.ContextLogf(ctx, "Copy %q", text)

		if err := clearInputField(ctx, res, addressBarNode); err != nil {
			return errors.Wrap(err, "failed to clear input field")
		}

		if err := uiauto.Combine(fmt.Sprintf("type and copy %q", text),
			res.ui.LeftClick(addressBarNode),
			res.ui.WaitUntilExists(addressBarNode.Focused()),
			res.kb.TypeAction(text),
			res.ui.WaitForLocation(addressBarNode),
			res.kb.AccelAction("Ctrl+A"),
			res.kb.AccelAction("Ctrl+C"),
		)(ctx); err != nil {
			return err
		}
	}
	return nil
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

type webpage struct {
	conn *chrome.Conn
}

func (b *webpage) openApp(ctx context.Context, res *clipboardResource) error {
	conn, err := res.br.NewConn(ctx, "")
	if err != nil {
		return errors.Wrap(err, "failed to connect to chrome")
	}
	b.conn = conn
	return nil
}

func (b *webpage) closeApp(ctx context.Context) error {
	if b.conn != nil {
		if err := b.conn.CloseTarget(ctx); err != nil {
			return err
		}
		return b.conn.Close()
	}
	return nil
}

func (b *webpage) pasteAndVerify(ctx context.Context, res *clipboardResource) error {
	rootView := nodewith.NameStartingWith("about:blank").HasClass("BrowserRootView")
	searchbox := nodewith.Role(role.TextField).Name("Address and search bar").Ancestor(rootView)
	return pasteAndVerify(ctx, res, searchbox, true /*clearField*/)
}

type setting struct {
	app *ossettings.OSSettings
}

func (set *setting) openApp(ctx context.Context, res *clipboardResource) error {
	settings, err := ossettings.Launch(ctx, res.tconn)
	if err != nil {
		return errors.Wrap(err, "failed  to launch OS settings")
	}
	set.app = settings
	return nil
}

func (set *setting) closeApp(ctx context.Context) error {
	if set.app != nil {
		return set.app.Close(ctx)
	}
	return nil
}

func (set *setting) pasteAndVerify(ctx context.Context, res *clipboardResource) error {
	return pasteAndVerify(ctx, res, ossettings.SearchBoxFinder, true /*clearField*/)
}

type gmail struct {
	conn *chrome.Conn
}

func (g *gmail) openApp(ctx context.Context, res *clipboardResource) error {
	conn, err := res.br.NewConn(ctx, "https://mail.google.com")
	if err != nil {
		return errors.Wrap(err, "failed to connect to chrome")
	}
	g.conn = conn
	return nil
}

func (g *gmail) closeApp(ctx context.Context) error {
	if g.conn != nil {
		if err := g.conn.CloseTarget(ctx); err != nil {
			return err
		}
		return g.conn.Close()
	}

	return nil
}

func (g *gmail) pasteAndVerify(ctx context.Context, res *clipboardResource) error {
	rootView := nodewith.Role(role.Window).NameContaining("Mail").HasClass("BrowserFrame")
	searchbox := nodewith.Role(role.TextField).Name("Search all conversations").Ancestor(rootView)
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

type launch struct {
	closeLauncher uiauto.Action
}

func (l *launch) openApp(ctx context.Context, res *clipboardResource) error {
	l.closeLauncher = launcher.CloseBubbleLauncher(res.tconn)
	return launcher.OpenBubbleLauncher(res.tconn)(ctx)
}

func (l *launch) closeApp(ctx context.Context) error {
	return l.closeLauncher(ctx)
}

func (l *launch) pasteAndVerify(ctx context.Context, res *clipboardResource) error {
	search := nodewith.HasClass("SearchBoxView")
	searchbox := nodewith.HasClass("Textfield").Role(role.TextField).Ancestor(search)
	return pasteAndVerify(ctx, res, searchbox, true /*clearField*/)
}

type fileapp struct {
	app *filesapp.FilesApp
}

func (f *fileapp) openApp(ctx context.Context, res *clipboardResource) error {
	files, err := filesapp.Launch(ctx, res.tconn)
	if err != nil {
		return errors.Wrap(err, "failed to launch filesapp")
	}
	f.app = files
	return nil
}

func (f *fileapp) closeApp(ctx context.Context) error {
	if f.app != nil {
		return f.app.Close(ctx)
	}
	return nil
}

func (f *fileapp) pasteAndVerify(ctx context.Context, res *clipboardResource) error {
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
