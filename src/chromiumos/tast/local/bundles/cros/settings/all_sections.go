// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package settings

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AllSections,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Open OS Settings and check main sections are displayed properly",
		Contacts:     []string{"tim.chang@cienet.com", "cienet-development@googlegroups.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

// AllSections goes through all main sections of OS settings.
func AllSections(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	if err := quicksettings.OpenSettingsApp(ctx, tconn); err != nil {
		s.Fatal("Failed to open OS settings: ", err)
	}
	osSettings := ossettings.New(tconn)
	defer osSettings.Close(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	if err := osSettings.WaitForSearchBox()(ctx); err != nil {
		s.Fatal("Failed to wait for OS-settings is ready to use: ", err)
	}

	if err := expandSubSection(osSettings, ossettings.Advanced, true)(ctx); err != nil {
		s.Fatal("Failed to expand advanced settings: ", err)
	}

	sections := ossettings.CommonSections(true)
	for sectionName, sectionFinder := range sections {
		section := sectionTest{
			name:   sectionName,
			finder: sectionFinder,
		}
		if sectionFinder == ossettings.Network {
			section.subSectionToExpand = "Add network connection"
		}
		if sectionFinder == ossettings.DateAndTime {
			section.subSettingToToggle = "Use 24-hour clock"
		}

		if err := checkSection(ctx, cr, osSettings, section); err != nil {
			s.Fatalf("Failed to check section %s: %v", sectionName, err)
		}
	}
}

type sectionTest struct {
	name               string
	finder             *nodewith.Finder
	subSectionToExpand string
	subSettingToToggle string
}

// checkSection checks section within the ossettings, and verifies specified subsection or sub-setting is properly displayed by expands/toggles on it.
func checkSection(ctx context.Context, cr *chrome.Chrome, osSettings *ossettings.OSSettings, section sectionTest) error {
	if err := uiauto.Combine(fmt.Sprintf("look for section: %q", section.name),
		ensureVisible(osSettings, section.finder),
		osSettings.WaitUntilExists(section.finder.Onscreen()),
	)(ctx); err != nil {
		return err
	}

	if section.subSectionToExpand != "" {
		node := nodewith.Name(section.subSectionToExpand).Role(role.Button)
		if err := osSettings.LeftClick(section.finder)(ctx); err != nil {
			return errors.Wrapf(err, "failed to navigate to section %q", section.name)
		}
		if err := expandSubSection(osSettings, node, true)(ctx); err != nil {
			return errors.Wrap(err, "failed to expand sub section")
		}
	}

	if section.subSettingToToggle != "" {
		if err := osSettings.LeftClick(section.finder)(ctx); err != nil {
			return errors.Wrapf(err, "failed to navigate to section %q", section.name)
		}
		if err := toggleSetting(cr, osSettings, section.subSettingToToggle)(ctx); err != nil {
			return errors.Wrap(err, "failed to toggle setting")
		}
	}

	return nil
}

func expandSubSection(osSettings *ossettings.OSSettings, node *nodewith.Finder, expected bool) uiauto.Action {
	return uiauto.Combine(fmt.Sprintf("expand sub section: %s", node.Pretty()),
		ensureFocused(osSettings, node),
		osSettings.LeftClick(node.State(state.Expanded, !expected)),
		osSettings.WaitUntilExists(node.State(state.Expanded, expected)),
	)
}

func toggleSetting(cr *chrome.Chrome, osSettings *ossettings.OSSettings, name string) uiauto.Action {
	return uiauto.Combine(fmt.Sprintf("toggle setting: %s", name),
		osSettings.SetToggleOption(cr, name, true),
		osSettings.SetToggleOption(cr, name, false),
	)
}

func ensureVisible(osSettings *ossettings.OSSettings, node *nodewith.Finder) uiauto.Action {
	return func(ctx context.Context) error {
		if found, err := osSettings.IsNodeFound(ctx, nodewith.Role(role.Navigation).First()); err != nil {
			return errors.Wrap(err, "failed to try to find node")
		} else if !found {
			// The main menu might be collapsed depending on window size, expand the main menu to ensure the input node is visible.
			if err = osSettings.LeftClick(ossettings.MenuButton)(ctx); err != nil {
				return errors.Wrap(err, "failed to click menu button")
			}
		}

		info, err := osSettings.Info(ctx, node)
		if err != nil {
			return err
		}
		if !info.State[state.Offscreen] {
			return nil
		}
		return osSettings.MakeVisible(node)(ctx)
	}
}

func ensureFocused(osSettings *ossettings.OSSettings, node *nodewith.Finder) uiauto.Action {
	return func(ctx context.Context) error {
		info, err := osSettings.Info(ctx, node)
		if err != nil {
			return err
		}
		if info.State[state.Focused] {
			return nil
		}
		return osSettings.FocusAndWait(node)(ctx)
	}
}
