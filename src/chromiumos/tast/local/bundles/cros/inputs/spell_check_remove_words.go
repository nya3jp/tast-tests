// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SpellCheckRemoveWords,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Verify that spell check works while typing the word removed from customize spell check",
		Contacts: []string{
			"cj.tsai@cienet.com",  // Author
			"shengjun@google.com", // PoC
			"cienet-development@googlegroups.com",
			"essential-inputs-team@google.com",
		},
		Attr:         []string{"group:mainline", "informational", "group:input-tools"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
		Timeout: 3 * time.Minute,
	})
}

// SpellCheckRemoveWords verifies that spell check works while typing the word removed from customize spell check.
func SpellCheckRemoveWords(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to take keyboard: ", err)
	}
	defer kb.Close()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	ui := uiauto.New(tconn)
	settings, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "osLanguages/input", ui.WaitUntilExists(ossettings.LanguagesAndInputs))
	if err != nil {
		s.Fatal("Failed to open the OS settings page: ", err)
	}
	defer settings.Close(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump_settings")

	s.Log("Enabling spelling and grammar check")
	if err := settings.SetToggleOption(cr, "Spelling and grammar check", true)(ctx); err != nil {
		s.Fatal("Failed to toggle spelling and grammar check: ", err)
	}

	const customizedWord = "newword"
	s.Log("Adding customized word: ", customizedWord)
	customizeSpellCheckLink := nodewith.Name("Customize spell check").Role(role.Link)
	addWordsTextField := nodewith.Name("Add words you want spell check to skip").Role(role.TextField)
	deleteWordNode := nodewith.Name("Delete word").HasClass("icon-clear").Role(role.Button)
	if err := uiauto.Combine("add word into customize spell check",
		settings.FocusAndWait(customizeSpellCheckLink),
		settings.LeftClick(customizeSpellCheckLink),
		settings.FocusAndWait(addWordsTextField),
		kb.TypeAction(customizedWord),
		settings.LeftClick(nodewith.Name("Add word").Role(role.Button)),
		// The added word doesn't have a name related to the word.
		// Verify that the word is added by checking the node named `Delete word` exists.
		settings.WaitUntilExists(deleteWordNode),
	)(ctx); err != nil {
		s.Fatal("Failed to complete all actions: ", err)
	}

	// Click the `Delete word` node until the node gone to make sure the added word is removed.
	s.Log("Removing customized word: ", customizedWord)
	if err := settings.LeftClickUntil(deleteWordNode, settings.Gone(deleteWordNode))(ctx); err != nil {
		s.Fatal("Failed to click delete word button: ", err)
	}

	browserType := s.Param().(browser.Type)
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, browserType)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	url := "data:text/html, <textarea aria-label='textarea'/>"
	browserNodeFinder := nodewith.Ancestor(nodewith.Role(role.Window).HasClass("BrowserFrame").NameContaining(url))
	if browserType == browser.TypeLacros {
		classNameRegexp := regexp.MustCompile(`^ExoShellSurface(-\d+)?$`)
		browserNodeFinder = nodewith.Ancestor(nodewith.Role(role.Window).ClassNameRegex(classNameRegexp).NameContaining(url))
	}

	conn, err := br.NewConn(ctx, url)
	if err != nil {
		s.Fatal("Failed to open web page: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump_browser")

	// The red wavy underline is not available on the UI tree.
	// It will show spelling suggestions and `Add to dictionary` options if we right click on the word with red wavy underline.
	s.Logf("Typing a misspelling word %q and checks the spelling suggestion", customizedWord)
	textArea := browserNodeFinder.Name("textarea").Role(role.TextField)
	menuContainer := nodewith.Ancestor(nodewith.Role(role.Menu).HasClass("SubmenuView"))
	addToDictionary := menuContainer.Name("Add to dictionary").HasClass("MenuItemView").Role(role.MenuItem)
	if err := uiauto.Combine("check for spelling check suggestion",
		ui.FocusAndWait(textArea),
		kb.TypeAction(customizedWord),
		kb.AccelAction("Ctrl+A"),
		ui.RightClick(textArea),
		ui.WaitUntilExists(addToDictionary),
	)(ctx); err != nil {
		s.Fatal("Failed to complete all actions: ", err)
	}
}
