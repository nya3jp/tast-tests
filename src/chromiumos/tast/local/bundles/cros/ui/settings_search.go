// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const uiWaitTime = 10 * time.Second

type search struct {
	keyword            string
	expectedRes        string
	category           searchType
	deepLinkingSection string
	subpageLabel       string
}

type searchType int

const (
	common searchType = iota
	checkSubpage
	deepLinking
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SettingsSearch,
		Desc:         "Search with keywords and verify the related results from OS Settings",
		Contacts:     []string{"tim.chang@cienet.com", "cienet-development@googlegroups.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		// ARC must be enabled to get search result 'Android preferences' by searching with keyword Android.
		Fixture: "arcBootedWithoutUIAutomator",
	})
}

func SettingsSearch(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "settings_search")

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	ui := uiauto.New(tconn)

	const (
		unifiedSystemTray = "UnifiedSystemTray"
		settingsText      = "Settings"
		settingsImage     = "TopShortcutButton"
	)

	// Open OS settings from system tray by UI operation instead of using ossettings.Launch().
	systemTray := nodewith.HasClass(unifiedSystemTray)
	settingsIcon := nodewith.Name(settingsText).HasClass(settingsImage)

	if err := uiauto.Combine("open settings from the system tray",
		ui.LeftClickUntil(systemTray, ui.WithTimeout(uiWaitTime).WaitUntilExists(settingsIcon)),
		ui.LeftClickUntil(settingsIcon, ui.WithTimeout(uiWaitTime).WaitUntilExists(ossettings.SearchBoxFinder)),
		ui.WaitUntilExists(ossettings.SearchBoxFinder.Focused()),
	)(ctx); err != nil {
		s.Fatal("Failed to open OS settings and check its state: ", err)
	}

	searchWords := []search{
		{keyword: "wifi", expectedRes: "Wi-Fi", category: checkSubpage, subpageLabel: "Known networks"},

		{keyword: "printters", expectedRes: "printer", category: common},
		{keyword: "turnoff", expectedRes: "Turn off", category: common},
		{keyword: "android", expectedRes: "Android", category: common},
		{keyword: "plays", expectedRes: "Play", category: common},
		{keyword: "photo", expectedRes: "photo", category: common},
		{keyword: "wall", expectedRes: "wall", category: common},
		{keyword: "acces", expectedRes: "Accessibility", category: common},
		{keyword: "Drive", expectedRes: "Drive", category: common},
		{keyword: "keyboa", expectedRes: "Keyboard", category: common},
		{keyword: "langu", expectedRes: "Languages", category: common},
		{keyword: "Input", expectedRes: "Inputs", category: common},
		{keyword: "gpu", expectedRes: "No search results found", category: common},

		{keyword: "on-screen", expectedRes: "On-screen", category: deepLinking, deepLinkingSection: "Enable on-screen keyboard"},
		{keyword: "Night light", expectedRes: "Night Light", category: deepLinking, deepLinkingSection: "Night Light"},
		{keyword: "high contrast", expectedRes: "High contrast", category: deepLinking, deepLinkingSection: "Use high contrast mode"},
	}

	for _, search := range searchWords {

		switch search.category {
		case common:
			if _, err := searchWithKeywords(ctx, ui, kb, search); err != nil {
				s.Fatal("Failed to search with keyword: ", err)
			}
		case checkSubpage:
			if err := searchAndCheckSubpage(ctx, ui, kb, search); err != nil {
				s.Fatal("Failed to search and check subpage: ", err)
			}
		case deepLinking:
			if err := searchForDeepLinking(ctx, ui, kb, search); err != nil {
				s.Fatal("Failed to search for deep linking: ", err)
			}
		}

		if err := clearSearch(ctx, ui); err != nil {
			s.Fatal("Failed to clear search: ", err)
		}
	}

}

func searchAndCheckSubpage(ctx context.Context, ui *uiauto.Context, kb *input.KeyboardEventWriter, searchIns search) error {

	firstOption, err := searchWithKeywords(ctx, ui, kb, searchIns)
	if err != nil {
		return errors.Wrap(err, "failed to search and check subpage")
	}

	if err := ui.LeftClickUntil(
		firstOption,
		ui.WithTimeout(uiWaitTime).WaitUntilExists(nodewith.Name(searchIns.subpageLabel)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to enter the corresponding subpage for choosen option")
	}

	return nil
}

func searchForDeepLinking(ctx context.Context, ui *uiauto.Context, kb *input.KeyboardEventWriter, searchIns search) error {

	matchedResult, err := searchWithKeywords(ctx, ui, kb, searchIns)
	if err != nil {
		return errors.Wrap(err, "failed to search for deep linking")
	}
	// Click and display results with focus section of deep linking.
	if err := ui.LeftClickUntil(
		matchedResult,
		ui.WithTimeout(uiWaitTime).WaitUntilExists(nodewith.Name(searchIns.deepLinkingSection).Role(role.ToggleButton)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to enter the exact section by deep linking")
	}

	return nil
}

func clearSearch(ctx context.Context, ui *uiauto.Context) error {

	const clearSearch = "Clear search"

	clearSearchBtn := nodewith.NameContaining(clearSearch).Role(role.Button)
	return ui.LeftClickUntil(
		clearSearchBtn,
		ui.WithTimeout(uiWaitTime).WaitUntilGone(clearSearchBtn),
	)(ctx)
}

func searchWithKeywords(ctx context.Context, ui *uiauto.Context, kb *input.KeyboardEventWriter, searchIns search) (*nodewith.Finder, error) {

	const resultsView = "ContentsWebView"

	searchRole := role.GenericContainer
	if searchIns.keyword == "gpu" {
		searchRole = role.StaticText
	}

	// Check the matching results.
	searchResults := nodewith.NameRegex(regexp.MustCompile("Search result|" + searchIns.expectedRes + "")).Role(searchRole)
	firstResult := searchResults.First()

	if err := uiauto.Combine("query with keywords '"+searchIns.keyword+"' and check the results view",
		kb.TypeAction(searchIns.keyword),
		ui.WaitUntilExists(nodewith.HasClass(resultsView).State(state.Focused, true)),
		// Wait for the first element in case the results view is not fully updated.
		ui.WaitUntilExists(firstResult),
	)(ctx); err != nil {
		return nil, err
	}

	resultsNodes, err := ui.NodesInfo(ctx, searchResults)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get NodesInfo related to search results of '%q'", searchIns.expectedRes)
	}

	// Check all of matching results.
	for _, result := range resultsNodes {
		// Last search results sometimes are kept here from the last search keyword.
		// It seems the data are out of sync from NodesInfo and UI rendering.
		if err := ui.WithTimeout(uiWaitTime).WaitUntilExists(nodewith.Name(result.Name).First())(ctx); err != nil {
			return nil, errors.Wrapf(err,
				"failed to find the matching results with keywords %q, expected result name: %s",
				searchIns.keyword, result.Name,
			)
		}
	}
	return firstResult, nil
}
