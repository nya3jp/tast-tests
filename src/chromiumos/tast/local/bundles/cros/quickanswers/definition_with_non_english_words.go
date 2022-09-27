// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package quickanswers

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/quickanswers"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DefinitionWithNonEnglishWords,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test Quick Answers definition feature on non-English words",
		Contacts: []string{
			"updowndota@google.com",
			"croissant-eng@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val: browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
	})
}

// DefinitionWithNonEnglishWords tests Quick Answers definition feature on non-English words.
func DefinitionWithNonEnglishWords(ctx context.Context, s *testing.State) {
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Setup chrome session with the Quick Answers for more locales feature flag enabled for any browser.
	bt := s.Param().(browser.Type)
	cr, br, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, bt,
		lacrosfixt.NewConfig(lacrosfixt.ChromeOptions(chrome.LacrosEnableFeatures("QuickAnswersForMoreLocales"))),
		chrome.EnableFeatures("QuickAnswersForMoreLocales"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)
	defer closeBrowser(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	ui := uiauto.New(tconn)

	if err := quickanswers.SetPrefValue(ctx, tconn, "settings.quick_answers.enabled", true); err != nil {
		s.Fatal("Failed to enable Quick Answers: ", err)
	}

	const languagesList = "en,es,it,fr,pt,de"

	if err := quickanswers.SetPrefValue(ctx, tconn, "settings.language.preferred_languages", languagesList); err != nil {
		s.Fatal("Failed to set preferred languages: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	for _, query := range []struct {
		queryWord       string
		languageName    string
		languageCode    string
		responseKeyword string
	}{
		{
			queryWord:       "pentágono",
			languageName:    "Spanish",
			languageCode:    "es",
			responseKeyword: "cinco lados",
		},
		{
			queryWord:       "settimana",
			languageName:    "Italian",
			languageCode:    "it",
			responseKeyword: "sette giorni",
		},
		{
			queryWord:       "semaine",
			languageName:    "French",
			languageCode:    "fr",
			responseKeyword: "sept jours",
		},
		{
			queryWord:       "futebol",
			languageName:    "Portuguese",
			languageCode:    "pt",
			responseKeyword: "11 jogadores",
		},
		{
			queryWord:       "verdreifachen",
			languageName:    "German",
			languageCode:    "de",
			responseKeyword: "dreimal",
		},
	} {
		s.Log("Testing definition query with " + query.languageName + " word: " + query.queryWord)

		// Open page with the query word on it.
		conn, err := br.NewConn(ctx, "https://google.com/search?q="+query.queryWord)
		if err != nil {
			s.Fatal("Failed to create new Chrome connection: ", err)
		}
		defer conn.Close()
		defer conn.CloseTarget(ctx)

		// Wait for the query word to appear.
		queryText := nodewith.Name(query.queryWord).Role(role.StaticText).First()
		if err := ui.WaitUntilExists(queryText)(ctx); err != nil {
			s.Fatal("Failed to wait for query to load: ", err)
		}

		// Right click the selected word and ensure the Quick Answers UI shows up with the definition result.
		quickAnswers := nodewith.ClassName("QuickAnswersView")
		definitionResult := nodewith.NameContaining(query.responseKeyword).ClassName("QuickAnswersTextLabel")
		if err := uiauto.Combine("Show context menu",
			ui.RightClick(queryText),
			ui.WaitUntilExists(quickAnswers),
			ui.WaitUntilExists(definitionResult))(ctx); err != nil {
			s.Fatal("Quick Answers result not showing up: ", err)
		}

		// Dismiss the context menu and ensure the Quick Answers UI also dismiss.
		if err := uiauto.Combine("Dismiss context menu",
			kb.AccelAction("Esc"),
			ui.WaitUntilGone(quickAnswers))(ctx); err != nil {
			s.Fatal("Quick Answers result not dismissed: ", err)
		}
	}
}
