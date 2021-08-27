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

	// Launch ossettings by UI control from quicksettings (uber-tray).
	if err := quicksettings.OpenSettingsApp(ctx, tconn); err != nil {
		s.Fatal("Failed to open OS settings: ", err)
	}
	osSettings := ossettings.New(tconn)
	defer func(ctx context.Context) {
		faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_dump")
		osSettings.Close(ctx)
	}(cleanupCtx)

	sections := map[string]*checkSectionsTest{
		"Network":            {ossettings.Network, "Add network connection", ""},
		"Bluetooth":          {ossettings.Bluetooth, "", ""},
		"ConnectedDevices":   {ossettings.ConnectedDevices, "", ""},
		"Accounts":           {ossettings.Accounts, "", ""},
		"Device":             {ossettings.Device, "", ""},
		"Personalization":    {ossettings.Personalization, "", ""},
		"SearchAndAssistant": {ossettings.SearchAndAssistant, "", ""},
		"SecurityAndPrivacy": {ossettings.SecurityAndPrivacy, "", ""},
		"Apps":               {ossettings.Apps, "", ""},
		"AboutChromeOS":      {ossettings.AboutChromeOS, "", ""},
	}
	if err := checkSections(ctx, cr, osSettings, &sections); err != nil {
		s.Fatal("Failed to go through each main sections: ", err)
	}

	if err := expandSubSection(osSettings, ossettings.Advanced, true)(ctx); err != nil {
		s.Fatal("Failed to expand advanced settings: ", err)
	}

	sections = map[string]*checkSectionsTest{
		"DateAndTime":        {ossettings.DateAndTime, "", "Use 24-hour clock"},
		"LanguagesAndInputs": {ossettings.LanguagesAndInputs, "", ""},
		"Files":              {ossettings.Files, "", ""},
		"PrintAndScan":       {ossettings.PrintAndScan, "", ""},
		"Developers":         {ossettings.Developers, "", ""},
		"Accessibility":      {ossettings.Accessibility, "", ""},
		"ResetSettings":      {ossettings.ResetSettings, "", ""},
	}
	if err := checkSections(ctx, cr, osSettings, &sections); err != nil {
		s.Fatal("Failed to go through each advanced sections: ", err)
	}
}

type checkSectionsTest struct {
	sectionNode *nodewith.Finder

	nodeToExpandSubSection string
	nodeToToggleSetting    string
}

func checkSections(ctx context.Context, cr *chrome.Chrome, osSettings *ossettings.OSSettings, sections *map[string]*checkSectionsTest) error {
	for name, section := range *sections {
		if err := uiauto.Combine(fmt.Sprintf("looking for section: %q", name),
			ensureVisible(osSettings, section.sectionNode),
			osSettings.WaitUntilExists(section.sectionNode.Onscreen()),
		)(ctx); err != nil {
			return err
		}
		testing.ContextLogf(ctx, "Secion: %q is displayed properly", name)

		if section.nodeToExpandSubSection != "" {
			node := nodewith.Name(section.nodeToExpandSubSection).Role(role.Button)
			if err := expandSubSection(osSettings, node, true)(ctx); err != nil {
				return errors.Wrap(err, "failed to expand sub section")
			}
			if err := expandSubSection(osSettings, node, false)(ctx); err != nil {
				return errors.Wrap(err, "failed to collapse sub section")
			}
		}
		if section.nodeToToggleSetting != "" {
			if err := toggleSetting(cr, osSettings, section.nodeToToggleSetting)(ctx); err != nil {
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
