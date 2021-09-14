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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/testing"
)

type ubertrayLayoutSubTests int

const (
	clamshell ubertrayLayoutSubTests = iota
	tablet
)

func init() {
	testing.AddTest(&testing.Test{
		Func: BasicUbertrayLayout,
		Desc: "Checks that settings can be found on Quick Settings",
		Contacts: []string{
			"ting.chen@cienet.com",
			"lance.wang@cienet.com",
			"cienet-development@googlegroups.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name: "clamshell",
				Val:  clamshell,
			}, {
				Name: "tablet",
				Val:  tablet,
			},
		},
	})
}

type ubertrayLayoutTestResources struct {
	cr        *chrome.Chrome
	tconn     *chrome.TestConn
	ui        *uiauto.Context
	pc        pointer.Context
	btn       *nodewith.Finder
	toggleBtn *nodewith.Finder
}

// BasicUbertrayLayout verifies that the basic components in Quick Settings are existed.
func BasicUbertrayLayout(ctx context.Context, s *testing.State) {
	const (
		username = "testuser"
		password = "1234567890"
	)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Fake login with specific account is used to verify the user's information.
	cr, err := chrome.New(ctx, chrome.FakeLogin(chrome.Creds{User: username, Pass: password}))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	ui := uiauto.New(tconn)

	isTablet := s.Param().(ubertrayLayoutSubTests) == tablet
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, isTablet)
	if err != nil {
		s.Fatal("Failed to enable the tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	var pc pointer.Context
	if isTablet {
		if pc, err = pointer.NewTouch(ctx, tconn); err != nil {
			s.Fatal("Failed to create touch context: ", err)
		}
	} else {
		pc = pointer.NewMouse(tconn)
	}
	defer pc.Close()

	resources := &ubertrayLayoutTestResources{
		cr:        cr,
		tconn:     tconn,
		ui:        ui,
		pc:        pc,
		btn:       nodewith.Role(role.Button),
		toggleBtn: nodewith.Role(role.ToggleButton),
	}

	s.Log("Enable accessibility and keyboard quick settings")
	if err := enableAccessAndKeyboard(ctx, resources); err != nil {
		s.Fatal("Failed to enable Accessibility and Keyboard: ", err)
	}

	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to open Quick Settings: ", err)
	}
	defer quicksettings.Hide(cleanupCtx, tconn)

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(cleanupCtx, cr, s.OutDir(), s.HasError)

	s.Log("Checking user in Quick Settings")
	if err := checkUser(ctx, resources, username); err != nil {
		s.Fatal("Failed to check user in Quick Settings: ", err)
	}

	s.Log("Checking buttons in Quick Settings")
	if err := checkButtons(ctx, resources); err != nil {
		s.Fatal("Failed to check buttons in Quick Settings: ", err)
	}

	s.Log("Checking panels in Quick Settings")
	if err := checkPannels(ctx, resources); err != nil {
		s.Fatal("Failed to check pannels in Quick Settings: ", err)
	}

	s.Log("Checking sliders in Quick Settings")
	if err := checkSliders(ctx, resources); err != nil {
		s.Fatal("Failed to check sliders in Quick Settings: ", err)
	}

	s.Log("Checking date in Quick Settings")
	if err := ui.WaitUntilExists(quicksettings.DateView)(ctx); err != nil {
		s.Fatal("Failed to find Date info: ", err)
	}

	s.Log("Checking battery in Quick Settings")
	if err := ui.WaitUntilExists(quicksettings.BatteryView)(ctx); err != nil {
		s.Fatal("Failed to find Battery info: ", err)
	}
}

func checkUser(ctx context.Context, res *ubertrayLayoutTestResources, username string) error {
	if err := uiauto.Combine("find user",
		res.pc.Click(res.btn.NameStartingWith(username)),
		res.ui.WaitUntilExists(nodewith.Role(role.StaticText).Name(username)),
		res.ui.WaitUntilExists(nodewith.Role(role.StaticText).Name(username+"@gmail.com")),
		res.ui.WaitUntilExists(nodewith.HasClass("RoundedImageView").First()),
		res.pc.Click(res.btn.Name("Close").HasClass("TopShortcutButton")),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to verify user info")
	}

	return nil
}

func checkButtons(ctx context.Context, res *ubertrayLayoutTestResources) error {
	for _, node := range []struct {
		finder    *nodewith.Finder
		name      string
		classname string
	}{
		{res.btn, "Sign out", "SignOutButton"},
		{res.btn, "Lock", "TopShortcutButton"},
		{res.btn, "Settings", "TopShortcutButton"},
		{res.btn, "Collapse menu", "CollapseButton"},
		{res.toggleBtn, "Toggle network", "FeaturePodIconButton"},
		{res.toggleBtn, "Toggle Bluetooth", "FeaturePodIconButton"},
		{res.toggleBtn, "Toggle Do not disturb", "FeaturePodIconButton"},
		{res.toggleBtn, "Toggle Night Light", "FeaturePodIconButton"},
		{res.toggleBtn, "Toggle Volume", "UnifiedSliderButton"},
	} {
		obj := node.finder.NameStartingWith(node.name).HasClass(node.classname)
		if err := res.ui.WaitUntilExists(obj)(ctx); err != nil {
			return errors.Wrapf(err, "failed to find button %q: ", node.name)
		}
		testing.ContextLogf(ctx, "Section %q found", node.name)
	}

	return nil
}

func checkPannels(ctx context.Context, res *ubertrayLayoutTestResources) error {
	panels := []string{
		"network",
		"Bluetooth",
		"notification",
		"accessibility settings",
		"keyboard settings",
	}

	for _, name := range panels {
		targetBtn := res.btn.NameStartingWith("Show " + name).HasClass("FeaturePodLabelButton")
		if err := uiauto.Combine("check panel",
			searchPannelInQuickSettings(res, targetBtn),
			res.pc.Click(targetBtn),
			res.pc.Click(res.btn.Name("Previous menu")),
		)(ctx); err != nil {
			return errors.Wrapf(err, "failed to check panel %q", name)
		}
		testing.ContextLogf(ctx, "Panel %q found", name)

		// Click on page one button if it exists.
		pageBtn := res.btn.NameRegex(regexp.MustCompile(`Page 1 of \d+`)).HasClass("PageIndicatorView")
		if err := res.ui.IfSuccessThen(
			res.ui.WithTimeout(3*time.Second).WaitUntilExists(pageBtn),
			res.pc.Click(pageBtn),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to back to first page")
		}
	}
	return nil
}

func searchPannelInQuickSettings(res *ubertrayLayoutTestResources, targetBtn *nodewith.Finder) uiauto.Action {
	pageIndicator := res.btn.NameRegex(regexp.MustCompile(`Page \d+ of \d+`)).HasClass("PageIndicatorView")

	return uiauto.NamedAction("search panels in pages", func(ctx context.Context) error {
		for nth := 1; ; nth++ {
			if err := res.ui.WaitForLocation(targetBtn)(ctx); err != nil {
				return errors.Wrap(err, "failed to wait for node stable")
			}

			info, err := res.ui.Info(ctx, targetBtn)
			if err != nil {
				return errors.Wrap(err, "failed to get node info")
			}

			if !info.State[state.Offscreen] {
				return nil
			}

			infos, err := res.ui.NodesInfo(ctx, pageIndicator)
			if err != nil {
				return errors.Wrap(err, "failed to get page indicator info")
			} else if nth >= len(infos) {
				return errors.Wrapf(err, "index out of bound [%d, %d)", nth, len(infos))
			}

			if err := res.pc.Click(pageIndicator.Nth(nth))(ctx); err != nil {
				return errors.Wrap(err, "failed to click page indicator")
			}
			testing.ContextLog(ctx, "Click next page")
		}
	})
}

func checkSliders(ctx context.Context, res *ubertrayLayoutTestResources) error {
	for _, sliders := range []struct {
		parentSection *nodewith.Finder
		targetNode    *nodewith.Finder
	}{
		{nil, quicksettings.VolumeSlider},
		{nil, quicksettings.BrightnessSlider},
		{res.btn.Name("Audio settings"), quicksettings.MicGainSlider},
	} {
		if sliders.parentSection != nil {
			if err := res.pc.Click(sliders.parentSection)(ctx); err != nil {
				return errors.Wrap(err, "failed to click parent section of a slider")
			}
		}

		if err := res.ui.WaitUntilExists(sliders.targetNode)(ctx); err != nil {
			return errors.Wrap(err, "failed to find slider")
		}

		if sliders.parentSection != nil {
			if err := res.pc.Click(res.btn.Name("Previous menu"))(ctx); err != nil {
				return errors.Wrap(err, "failed to go back to previous")
			}
		}
	}

	return nil
}

// enableAccessAndKeyboard enables accessibility and keyboard quick settings.
func enableAccessAndKeyboard(ctx context.Context, res *ubertrayLayoutTestResources) error {
	setting, err := ossettings.LaunchAtPageURL(ctx, res.tconn, res.cr, "osAccessibility", func(context.Context) error { return nil })
	if err != nil {
		return errors.Wrap(err, "failed to open setting page")
	}
	defer setting.Close(ctx)

	optionName := "Always show accessibility options in the system menu"
	if err := uiauto.Combine("add input methods",
		res.ui.WaitUntilExists(nodewith.Name(optionName).Role(role.ToggleButton)),
		setting.SetToggleOption(res.cr, optionName, true),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to toggle accessibility settings")
	}

	if err := setting.NavigateToPageURL(ctx, res.cr, "osLanguages", res.pc.Click(nodewith.NameStartingWith("Inputs"))); err != nil {
		return errors.Wrap(err, "failed to enter inputs settings page")
	}

	return uiauto.Combine("add input methods",
		res.pc.Click(nodewith.Role(role.Button).Name("Add input methods")),
		res.pc.Click(nodewith.Role(role.CheckBox).First()),
		res.pc.Click(nodewith.Role(role.Button).Name("Add")),
		res.ui.WaitUntilExists(nodewith.HasClass("list-item").First()),
	)(ctx)
}
