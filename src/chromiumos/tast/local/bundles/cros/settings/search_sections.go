// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package settings

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type settingsSearchTestParams struct {
	arc        bool
	searchtype settingsSearchType
}

type settingsSearchType int

const (
	normaloptions settingsSearchType = iota
	arcOptions
	optionsAndSubpage
	optionsAndDeepLinking
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SearchSections,
		Desc:         "Search with keywords and verify the related results from OS Settings",
		Contacts:     []string{"tim.chang@cienet.com", "cienet-development@googlegroups.com, chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Params: []testing.Param{
			{
				Name: "normal_options",
				Val: settingsSearchTestParams{
					arc:        false,
					searchtype: normaloptions,
				},
				Fixture: "chromeLoggedIn",
			}, {
				Name: "arc_options",
				Val: settingsSearchTestParams{
					arc:        true,
					searchtype: arcOptions,
				},
				Fixture: "arcBootedWithoutUIAutomator",
			}, {
				Name: "options_and_subpage",
				Val: settingsSearchTestParams{
					arc:        false,
					searchtype: optionsAndSubpage,
				},
				Fixture: "chromeLoggedIn",
			}, {
				Name: "options_and_deep_linking",
				Val: settingsSearchTestParams{
					arc:        false,
					searchtype: optionsAndDeepLinking,
				},
				Fixture: "chromeLoggedIn",
			},
		},
	})
}

type settingsSearchDetail struct {
	keyword string

	expectedResult     string
	expectedMismatch   bool
	expectedResultRole role.Role

	deepLinkingSection string
	subpageLabel       string
}

func searchDetail(st settingsSearchType) *[]settingsSearchDetail {
	switch st {
	case normaloptions:
		return &[]settingsSearchDetail{
			{
				keyword:            "turnoff",
				expectedResult:     `Turn off (Bluetooth|Wi\-Fi|networks)`,
				expectedResultRole: role.GenericContainer,
			}, {
				keyword:            "plays",
				expectedResult:     `Display`,
				expectedResultRole: role.GenericContainer,
			}, {
				keyword:            "photo",
				expectedResult:     `.*photos.*`,
				expectedResultRole: role.GenericContainer,
			}, {
				keyword:            "wall",
				expectedResult:     `Change wallpaper`,
				expectedResultRole: role.GenericContainer,
			}, {
				keyword:            "Drive",
				expectedResult:     `.*Drive.*`,
				expectedResultRole: role.GenericContainer,
			}, {
				keyword:            "Input",
				expectedResult:     `Inputs`,
				expectedResultRole: role.GenericContainer,
			}, {
				keyword:            "acces", // A incomplete keyword to search.
				expectedResult:     `Accessibility`,
				expectedResultRole: role.GenericContainer,
			}, {
				keyword:            "keyboa", // A incomplete keyword to search.
				expectedResult:     `Keyboard`,
				expectedResultRole: role.GenericContainer,
			}, {
				keyword:            "langu", // A incomplete keyword to search.
				expectedResult:     `Languages`,
				expectedResultRole: role.GenericContainer,
			}, {
				keyword:            "printters", // A typo keyword to search.
				expectedResult:     `Printers`,
				expectedResultRole: role.GenericContainer,
			}, {
				keyword:            "gpu",
				expectedResult:     `No search results found`,
				expectedResultRole: role.StaticText,
				expectedMismatch:   true,
			},
		}
	case arcOptions:
		return &[]settingsSearchDetail{
			{
				keyword:            "android",
				expectedResult:     `Android preferences`,
				expectedResultRole: role.GenericContainer,
			}, {
				keyword:            "android",
				expectedResult:     `Google Play Store`,
				expectedResultRole: role.GenericContainer,
			},
		}
	case optionsAndSubpage:
		return &[]settingsSearchDetail{
			{
				keyword:            "wifi",
				expectedResult:     `Wi-Fi networks`,
				expectedResultRole: role.GenericContainer,
				subpageLabel:       "Known networks",
			},
		}
	case optionsAndDeepLinking:
		return &[]settingsSearchDetail{
			{
				keyword:            "on-screen",
				expectedResult:     `On-screen keyboard`,
				expectedResultRole: role.GenericContainer,
				deepLinkingSection: "Enable on-screen keyboard",
			}, {
				keyword:            "Night light",
				expectedResult:     `Night Light`,
				expectedResultRole: role.GenericContainer,
				deepLinkingSection: `Night Light`,
			}, {
				keyword:            "high contrast",
				expectedResult:     `High contrast mode`,
				expectedResultRole: role.GenericContainer,
				deepLinkingSection: `Use high contrast mode`,
			},
		}
	default:
		return &[]settingsSearchDetail{}
	}
}

// SearchSections searches specified settings and checks corresponding section.
func SearchSections(ctx context.Context, s *testing.State) {
	params, ok := s.Param().(settingsSearchTestParams)
	if !ok {
		s.Fatal("Failed to get test parameters: ")
	}

	var cr *chrome.Chrome
	if params.arc {
		cr = s.FixtValue().(*arc.PreData).Chrome
	} else {
		cr = s.FixtValue().(*chrome.Chrome)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Launch ossettings by UI control from quicksettings (uber-tray).
	if err := quicksettings.OpenSettingsApp(ctx, tconn); err != nil {
		s.Fatal("Failed to open OS settings: ", err)
	}
	osSettings := ossettings.New(tconn)
	defer func(ctx context.Context) {
		faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_dump")
		osSettings.Close(ctx)
	}(cleanupCtx)

	// Verify that cursor is focus in the search field.
	if err := osSettings.WaitUntilExists(ossettings.SearchBoxFinder.Focused())(ctx); err != nil {
		s.Fatal("Failed to wait for cursor be focused on the search field in OS settings: ", err)
	}

	detail := *searchDetail(params.searchtype)
	for _, search := range detail {
		firstOption, err := searchWithKeywords(ctx, osSettings, kb, search)
		if err != nil {
			s.Fatal("Failed to search with keyword: ", err)
		}

		if search.subpageLabel != "" {
			if err := osSettings.WithTimeout(time.Minute).LeftClickUntil(
				firstOption,
				osSettings.WaitUntilExists(nodewith.Name(search.subpageLabel)),
			)(ctx); err != nil {
				s.Fatal("Failed to enter the corresponding subpage for choosen option: ", err)
			}
		}

		// Click and display results with focus section of deep linking.
		if search.deepLinkingSection != "" {
			if err := osSettings.WithTimeout(time.Minute).LeftClickUntil(
				firstOption,
				osSettings.WaitUntilExists(nodewith.NameRegex(regexp.MustCompile(search.deepLinkingSection)).Role(role.ToggleButton).Onscreen()),
			)(ctx); err != nil {
				s.Fatal("Failed to enter the exact section by deep linking: ", err)
			}
		}

		if err := clearSearch(osSettings)(ctx); err != nil {
			s.Fatal("Failed to clear search: ", err)
		}
	}
}

func clearSearch(osSettings *ossettings.OSSettings) uiauto.Action {
	clearSearchBtn := nodewith.NameContaining("Clear search").Role(role.Button)
	return osSettings.WithTimeout(time.Minute).LeftClickUntil(
		clearSearchBtn,
		osSettings.WaitUntilGone(clearSearchBtn),
	)
}

func searchWithKeywords(ctx context.Context, osSettings *ossettings.OSSettings, kb *input.KeyboardEventWriter,
	detail settingsSearchDetail) (*nodewith.Finder, error) {

	// Check the matching results.
	firstResult := nodewith.Role(detail.expectedResultRole).First()
	if !detail.expectedMismatch {
		// The results should show a minimum of 1 or maximum of 5 results.
		r := regexp.MustCompile(fmt.Sprintf(`Search result \d+ of [1-5]: %s`, detail.expectedResult))
		firstResult = firstResult.NameRegex(r).HasClass("no-outline")
	} else {
		firstResult = firstResult.Name(detail.expectedResult)
	}

	if err := uiauto.Combine(fmt.Sprintf("query with keywords %q and check the results view", detail.keyword),
		kb.TypeAction(detail.keyword),
		osSettings.WaitUntilExists(nodewith.HasClass("ContentsWebView").Focused()),
		osSettings.WaitUntilExists(firstResult), // Wait for search results be stabled.
	)(ctx); err != nil {
		return nil, err
	}

	return firstResult, nil
}
