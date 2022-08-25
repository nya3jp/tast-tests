// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package quicksettings

import (
	"context"
	"fmt"
	"regexp"
	"strings"
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
	"chromiumos/tast/testing/hwdep"
)

// basicLayoutTestParam is the parameter for BasicLayout tests.
type basicLayoutTestParam struct {
	// isTabletMode indicates whether the DUT should be in tablet mode.
	isTabletMode bool
	// hasBattery indicates if the DUT has builtin battery.
	hasBattery bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         BasicLayout,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that settings can be found on Quick Settings",
		Contacts: []string{
			"ting.chen@cienet.com",
			"lance.wang@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"cros-status-area-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Params: []testing.Param{
			{
				ExtraHardwareDeps: hwdep.D(hwdep.Battery()),
				Val: basicLayoutTestParam{
					isTabletMode: false,
					hasBattery:   true,
				},
			},
			{
				Name: "no_battery",
				Val: basicLayoutTestParam{
					isTabletMode: false,
					hasBattery:   false,
				},
			},
			{
				Name:              "tablet",
				ExtraHardwareDeps: hwdep.D(hwdep.Battery()),
				Val: basicLayoutTestParam{
					isTabletMode: true,
					hasBattery:   true,
				},
			},
			{
				Name: "tablet_no_battery",
				Val: basicLayoutTestParam{
					isTabletMode: true,
					hasBattery:   false,
				},
			},
		},
	})
}

type basicLayoutTestResources struct {
	cr     *chrome.Chrome
	tconn  *chrome.TestConn
	ui     *uiauto.Context
	pc     pointer.Context
	btn    *nodewith.Finder
	outDir string
}

// BasicLayout verifies that the basic components in Quick Settings are existed.
func BasicLayout(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	param := s.Param().(basicLayoutTestParam)
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, param.isTabletMode)
	if err != nil {
		s.Fatal("Failed to enable the tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	var pc pointer.Context
	if param.isTabletMode {
		if pc, err = pointer.NewTouch(ctx, tconn); err != nil {
			s.Fatal("Failed to create touch context: ", err)
		}
	} else {
		pc = pointer.NewMouse(tconn)
	}
	defer func() {
		if pc != nil {
			if err := pc.Close(); err != nil {
				testing.ContextLog(ctx, "Failed to close pointer context")
			}
		}
	}()

	resources := &basicLayoutTestResources{
		cr:     cr,
		tconn:  tconn,
		ui:     uiauto.New(tconn),
		pc:     pc,
		btn:    nodewith.Role(role.Button),
		outDir: s.OutDir(),
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
	if err := checkUser(ctx, resources); err != nil {
		s.Fatal("Failed to check user in Quick Settings: ", err)
	}

	s.Log("Checking buttons in Quick Settings")
	if err := checkButtons(ctx, resources); err != nil {
		s.Fatal("Failed to check buttons in Quick Settings: ", err)
	}

	s.Log("Checking panels in Quick Settings")
	if err := checkPanels(ctx, resources); err != nil {
		s.Fatal("Failed to check panels in Quick Settings: ", err)
	}

	s.Log("Checking sliders in Quick Settings")
	if err := checkSliders(ctx, resources); err != nil {
		s.Fatal("Failed to check sliders in Quick Settings: ", err)
	}

	s.Log("Checking date in Quick Settings")
	if err := resources.ui.WaitUntilExists(quicksettings.DateView)(ctx); err != nil {
		s.Fatal("Failed to find Date info: ", err)
	}

	// If DUT does not have battery (e.g., Chromeboxes and Chromebits),
	// skip battery icon verification.
	s.Log("Checking battery in Quick Settings")
	if param.hasBattery {
		if err := resources.ui.WaitUntilExists(quicksettings.BatteryView)(ctx); err != nil {
			s.Fatal("Failed to find Battery info: ", err)
		}
	}
}

func checkUser(ctx context.Context, res *basicLayoutTestResources) error {
	userEmail := res.cr.User()
	userName := strings.Split(userEmail, "@")[0]

	return uiauto.Combine(fmt.Sprintf("find user: %s, email: %s", userName, userEmail),
		res.pc.Click(res.btn.NameStartingWith(userName)),
		res.ui.WaitUntilExists(nodewith.Role(role.StaticText).Name(userName)),
		res.ui.WaitUntilExists(nodewith.Role(role.StaticText).Name(userEmail)),
		res.ui.WaitUntilExists(nodewith.HasClass("RoundedImageView").First()),
		res.pc.Click(res.btn.Name("Close").HasClass("IconButton")),
	)(ctx)
}

func checkButtons(ctx context.Context, res *basicLayoutTestResources) error {
	for _, node := range []*nodewith.Finder{
		quicksettings.SignoutButton,
		quicksettings.LockButton,
		quicksettings.SettingsButton,
		quicksettings.CollapseButton,
		quicksettings.VolumeToggle,
		quicksettings.PodIconButton(quicksettings.SettingPodNetwork),
		quicksettings.PodIconButton(quicksettings.SettingPodBluetooth),
		quicksettings.PodIconButton(quicksettings.SettingPodDoNotDisturb),
		quicksettings.PodIconButton(quicksettings.SettingPodNightLight),
	} {
		nodeInfo, err := res.ui.Info(ctx, node)
		if err != nil || nodeInfo == nil {
			return errors.Wrap(err, "failed to get node info")
		}
		testing.ContextLogf(ctx, "Button %q, %q found", nodeInfo.Name, nodeInfo.ClassName)
	}
	return nil
}

func checkPanels(ctx context.Context, res *basicLayoutTestResources) error {
	for _, pod := range []quicksettings.SettingPod{
		quicksettings.SettingPodNetwork,
		quicksettings.SettingPodBluetooth,
		quicksettings.SettingPodAccessibility,
		quicksettings.SettingPodKeyboard,
		quicksettings.SettingPodDoNotDisturb,
	} {
		podLabelBtn := quicksettings.PodLabelButton(pod)
		if err := uiauto.Combine("check panel",
			searchPanelInQuickSettings(res, podLabelBtn),
			res.pc.Click(podLabelBtn),
			res.pc.Click(res.btn.Name("Previous menu")),
		)(ctx); err != nil {
			return errors.Wrapf(err, "failed to check panel %q", string(pod))
		}
		testing.ContextLogf(ctx, "Panel %q found", string(pod))

		// Click on page one button if it exists.
		pageBtn := res.btn.NameRegex(regexp.MustCompile(`Page 1 of \d+`)).HasClass("PageIndicatorView")
		if err := uiauto.IfSuccessThen(
			res.ui.WithTimeout(3*time.Second).WaitUntilExists(pageBtn),
			res.pc.Click(pageBtn),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to back to first page")
		}
	}
	return nil
}

func searchPanelInQuickSettings(res *basicLayoutTestResources, targetBtn *nodewith.Finder) uiauto.Action {
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

func checkSliders(ctx context.Context, res *basicLayoutTestResources) error {
	for _, sliders := range []struct {
		parentSection *nodewith.Finder
		targetNode    *nodewith.Finder
	}{
		{nil, quicksettings.VolumeSlider},
		{nil, quicksettings.BrightnessSlider},
		// TODO(b/225081940): Enable the audio settings checks when the issue is fixed.
		// Audio details might be missing on some DUT models (e.g., kaisa, blacktip)
		// {res.btn.Name("Audio settings"), quicksettings.MicGainSlider},
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
func enableAccessAndKeyboard(ctx context.Context, res *basicLayoutTestResources) (retErr error) {
	setting, err := ossettings.LaunchAtPageURL(ctx, res.tconn, res.cr, "osAccessibility", func(context.Context) error { return nil })
	if err != nil {
		return errors.Wrap(err, "failed to open setting page")
	}
	defer setting.Close(ctx)
	defer faillog.DumpUITreeOnErrorToFile(ctx, res.outDir, func() bool { return retErr != nil }, res.tconn, "ui_dump_enable_access_and_keyboard")

	optionName := "Show accessibility options in Quick Settings"
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
		setting.SetToggleOption(res.cr, "Show input options in the shelf", false),
	)(ctx)
}
