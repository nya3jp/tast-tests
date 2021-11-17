// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package settings

import (
	"context"
	"math/rand"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
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
		Func:         AddAndRemoveLanguage,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check if it is able to add and remove language",
		Contacts:     []string{"vivian.tsai@cienet.com", "cienet-development@googlegroups.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Fixture:      "chromeLoggedIn",
	})
}

// AddAndRemoveLanguage adds and removes a selected language.
func AddAndRemoveLanguage(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create the keyboard: ", err)
	}
	defer kb.Close()

	ui := uiauto.New(tconn)
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	settings, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "osLanguages/languages", ui.Exists(nodewith.Name("Add languages").Role(role.Button)))
	if err != nil {
		s.Fatal("Failed to open language page: ", err)
	}
	defer settings.Close(cleanupCtx)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	targetLanguageFullName, err := findTargetLanguage(ctx, ui)
	if err != nil {
		s.Fatal("Failed to find target language: ", err)
	}

	if err := addNewLanguage(ui, targetLanguageFullName)(ctx); err != nil {
		s.Fatal("Failed to add new language: ", err)
	}

	// Expecting node with pattern "language - endonym", like:
	// "Afrikaans", "Albanian - shqip", "Serbo-Croatian - srpskohrvatski"
	r := regexp.MustCompile(`^(.*) - (.*)$`)
	ss := r.FindStringSubmatch(targetLanguageFullName)
	var targetLanguage string
	if ss != nil {
		if len(ss) != 3 {
			s.Fatalf("Unexpected language name %q: want 2 sub matches, got %d", targetLanguageFullName, len(ss))
		}
		// The left part is the desired language name.
		targetLanguage = ss[1]
	} else {
		// The full text is the language name if the endonym doesn't exists.
		targetLanguage = targetLanguageFullName
	}
	targetLanguageBtn := nodewith.Name(targetLanguage).Role(role.Button)

	s.Logf("Verifying the new language %q is added", targetLanguage)
	if err := ui.WaitUntilExists(targetLanguageBtn)(ctx); err != nil {
		s.Fatal("Failed to verify new language is added: ", err)
	}

	if err := removeNewLanguage(ui, targetLanguage)(ctx); err != nil {
		s.Fatal("Failed to remove language: ", err)
	}

	s.Logf("Verifying the language %q is removed", targetLanguage)
	if err := ui.WaitUntilGone(targetLanguageBtn)(ctx); err != nil {
		s.Fatal("Failed to verify language is removed: ", err)
	}
}

// findTargetLanguage finds target language's full name.
func findTargetLanguage(ctx context.Context, ui *uiauto.Context) (string, error) {
	checkbox := nodewith.Role(role.CheckBox).Ancestor(nodewith.Name("Settings - Languages").Role(role.RootWebArea))

	if err := uiauto.Combine("open languages list",
		ui.LeftClick(ossettings.AddLanguagesButton),
		ui.WaitUntilExists(ossettings.SearchLanguages),
	)(ctx); err != nil {
		return "", err
	}

	checkboxNode, err := ui.NodesInfo(ctx, checkbox)
	if err != nil {
		return "", errors.Wrap(err, "failed to get checkbox nodes' info")
	}

	checkboxName := checkboxNode[rand.Intn(len(checkboxNode))].Name

	return checkboxName, nil
}

// addNewLanguage adds new language to language menu.
func addNewLanguage(ui *uiauto.Context, targetLanguageFullName string) uiauto.Action {
	targetLanguageCheckBox := nodewith.Name(targetLanguageFullName).Role(role.CheckBox)

	return uiauto.Combine("add new language",
		ui.FocusAndWait(targetLanguageCheckBox),
		ui.LeftClick(targetLanguageCheckBox),
		ui.LeftClick(nodewith.Name("Add").Role(role.Button)),
	)
}

// removeNewLanguage removes new language from language menu.
func removeNewLanguage(ui *uiauto.Context, targetLanguage string) uiauto.Action {
	return uiauto.Combine("remove language",
		ui.LeftClick(nodewith.Name(targetLanguage).Role(role.Button)),
		ui.LeftClick(nodewith.Name("Remove").Role(role.MenuItem)),
	)
}
