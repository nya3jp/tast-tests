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
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AllSections,
		Desc:         "Open OS Settings and check main sections are displayed properly",
		Contacts:     []string{"tim.chang@cienet.com", "cienet-development@googlegroups.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
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
	defer func(ctx context.Context) {
		faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_dump")
		osSettings.Close(ctx)
	}(cleanupCtx)

	ui := uiauto.New(tconn)
	if err := ui.WaitUntilExists(ossettings.SearchBoxFinder.Focused())(ctx); err != nil {
		s.Fatal("Failed to wait for cursor be focused on the search field in OS settings: ", err)
	}

	if err := expandSubSection(osSettings, ossettings.Advanced, true)(ctx); err != nil {
		s.Fatal("Failed to expand advanced settings: ", err)
	}

	sections := ossettings.CommonSections(true)

	expandAndSubSection := map[*nodewith.Finder]string{
		ossettings.Network: "Add network connection",
	}

	toggleSetting := map[*nodewith.Finder]string{
		ossettings.DateAndTime: "Use 24-hour clock",
	}

	if err := checkSections(ctx, cr, osSettings, &sections, &expandAndSubSection, &toggleSetting); err != nil {
		s.Fatal("Failed to go through each main sections: ", err)
	}
}

func checkSections(ctx context.Context, cr *chrome.Chrome, osSettings *ossettings.OSSettings,
	sections *map[string]*nodewith.Finder, subSections, toggleSettings *map[*nodewith.Finder]string) error {

	for mainSecionName, mainSecion := range *sections {
		if err := uiauto.Combine(fmt.Sprintf("looking for section: %q", mainSecionName),
			ensureVisible(osSettings, mainSecion),
			osSettings.WaitUntilExists(mainSecion.Onscreen()),
		)(ctx); err != nil {
			return err
		}
		testing.ContextLogf(ctx, "Secion: %q is displayed properly", mainSecionName)

		if subSectionName, ok := (*subSections)[mainSecion]; ok {
			node := nodewith.Name(subSectionName).Role(role.Button)
			if err := osSettings.LeftClick(mainSecion)(ctx); err != nil {
				return errors.Wrapf(err, "failed to navigate to section %q", mainSecionName)
			}
			if err := expandSubSection(osSettings, node, true)(ctx); err != nil {
				return errors.Wrap(err, "failed to expand sub section")
			}
		}

		if toggleOptionName, ok := (*toggleSettings)[mainSecion]; ok {
			if err := osSettings.LeftClick(mainSecion)(ctx); err != nil {
				return errors.Wrapf(err, "failed to navigate to section %q", mainSecionName)
			}
			if err := toggleSetting(cr, osSettings, toggleOptionName)(ctx); err != nil {
				return errors.Wrap(err, "failed to toggle setting")
			}
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
