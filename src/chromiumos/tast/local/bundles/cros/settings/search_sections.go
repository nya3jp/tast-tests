// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package settings

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type settingsSearchTestParams struct {
	arc        bool
	searchtype settingsSearchType
}

type settingsSearchType int

const (
	normalOptions settingsSearchType = iota
	arcOptions
	optionsAndSubpage
	optionsAndDeepLinking
	guestMode
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SearchSections,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Search with keywords and verify the related results from OS Settings",
		Contacts:     []string{"tim.chang@cienet.com", "cienet-development@googlegroups.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "arc"},
		Params: []testing.Param{
			{
				Name: "normal_options",
				Val: settingsSearchTestParams{
					arc:        false,
					searchtype: normalOptions,
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
			}, {
				Name: "guest_mode",
				Val: settingsSearchTestParams{
					arc:        false,
					searchtype: guestMode,
				},
				Fixture: "chromeLoggedInGuest",
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

func searchDetail(st settingsSearchType) []settingsSearchDetail {
	switch st {
	case normalOptions:
		return []settingsSearchDetail{
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
				keyword:            "acces", // An incomplete keyword to search.
				expectedResult:     `Accessibility`,
				expectedResultRole: role.GenericContainer,
			}, {
				keyword:            "keyboa", // An incomplete keyword to search.
				expectedResult:     `Keyboard`,
				expectedResultRole: role.GenericContainer,
			}, {
				keyword:            "langu", // An incomplete keyword to search.
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
		return []settingsSearchDetail{
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
		return []settingsSearchDetail{
			{
				keyword:            "wifi",
				expectedResult:     `Wi-Fi networks`,
				expectedResultRole: role.GenericContainer,
				subpageLabel:       "Known networks",
			},
		}
	case optionsAndDeepLinking:
		return []settingsSearchDetail{
			{
				keyword:            "onscreen",
				expectedResult:     `On-screen keyboard`,
				expectedResultRole: role.GenericContainer,
				deepLinkingSection: "On-screen keyboard",
			}, {
				keyword:            "Night light",
				expectedResult:     `Night Light`,
				expectedResultRole: role.GenericContainer,
				deepLinkingSection: `Night Light`,
			}, {
				keyword:            "high contrast",
				expectedResult:     `High contrast mode`,
				expectedResultRole: role.GenericContainer,
				deepLinkingSection: `Color inversion`,
			},
		}
	case guestMode:
		return []settingsSearchDetail{
			{
				keyword:            "turnoff",
				expectedResult:     `Turn off (Bluetooth|Wi\-Fi|networks)`,
				expectedResultRole: role.GenericContainer,
			}, {
				keyword:            "photo",
				expectedResult:     `No search results found`,
				expectedResultRole: role.StaticText,
				expectedMismatch:   true,
			}, {
				keyword:            "wallpaper",
				expectedResult:     `No search results found`,
				expectedResultRole: role.StaticText,
				expectedMismatch:   true,
			}, {
				keyword:            "Drive",
				expectedResult:     `.*Drive.*`,
				expectedResultRole: role.GenericContainer,
			}, {
				keyword:            "Input",
				expectedResult:     `Inputs`,
				expectedResultRole: role.GenericContainer,
			},
		}
	default:
		return []settingsSearchDetail{}
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

	if err := quicksettings.OpenSettingsApp(ctx, tconn); err != nil {
		s.Fatal("Failed to open OS settings: ", err)
	}
	osSettings := ossettings.New(tconn)
	defer func() {
		faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")
		osSettings.Close(cleanupCtx)
	}()

	ui := uiauto.New(tconn)

	// Verify that cursor is focus in the search field.
	if err := ui.WaitUntilExists(ossettings.SearchBoxFinder.Focused())(ctx); err != nil {
		s.Fatal("Failed to wait for cursor be focused on the search field in OS settings: ", err)
	}

	for _, search := range searchDetail(params.searchtype) {
		result, err := searchAndCheck(ctx, osSettings, kb, search)
		if err != nil {
			s.Fatal("Failed to search with keyword: ", err)
		} else if result == nil {
			s.Fatal("Invalid search result")
		}

		if search.subpageLabel != "" {
			if err := ui.WithTimeout(30*time.Second).RetryUntil(
				mouse.Click(tconn, result.Location.CenterPoint(), mouse.LeftButton),
				osSettings.WithTimeout(10*time.Second).WaitUntilExists(nodewith.Name(search.subpageLabel)),
			)(ctx); err != nil {
				s.Fatal("Failed to enter the corresponding subpage for choosen option: ", err)
			}
		}

		// Click and display results with focus section of deep linking.
		if search.deepLinkingSection != "" {
			if err := ui.WithTimeout(30*time.Second).RetryUntil(
				mouse.Click(tconn, result.Location.CenterPoint(), mouse.LeftButton),
				osSettings.WithTimeout(10*time.Second).WaitUntilExists(nodewith.NameRegex(regexp.MustCompile(search.deepLinkingSection)).Role(role.ToggleButton).Onscreen()),
			)(ctx); err != nil {
				s.Fatal("Failed to enter the exact section by deep linking: ", err)
			}
		}

		if err := osSettings.ClearSearch()(ctx); err != nil {
			s.Fatal("Failed to clear search: ", err)
		}
	}
}

func searchAndCheck(ctx context.Context, osSettings *ossettings.OSSettings, kb *input.KeyboardEventWriter,
	detail settingsSearchDetail) (*uiauto.NodeInfo, error) {

	testing.ContextLogf(ctx, "Search for %q", detail.keyword)
	infos, mismatched, err := osSettings.SearchWithKeyword(ctx, kb, detail.keyword)
	if err != nil {
		return nil, err
	}

	// Verify search results count.
	if len(infos) == 0 {
		return nil, errors.New("no results found")
	} else if len(infos) > 5 || len(infos) < 1 {
		// The results should show a minimum of 1 or maximum of 5 results.
		return nil, errors.Errorf("unexpected result count, want: [1,5], got: %d", len(infos))
	}

	// Verify mismatch.
	if detail.expectedMismatch != mismatched {
		return nil, errors.Errorf("unexpected search result, want: [mismatch: %t], got: [mismatch: %t]", detail.expectedMismatch, mismatched)
	}

	// Verify result.
	rExpected := regexp.MustCompile(detail.expectedResult)
	for idx, info := range infos {
		if rExpected.MatchString(info.Name) {
			testing.ContextLogf(ctx, "Found: %q", infos[idx].Name)
			return &infos[idx], nil
		}
	}

	return nil, errors.Errorf("no match results found, the first result is %q", infos[0].Name)
}
