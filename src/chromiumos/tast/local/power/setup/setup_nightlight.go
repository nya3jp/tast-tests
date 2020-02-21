// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const uiTimeout = 30 * time.Second

func systemTray(ctx context.Context, tconn *chrome.Conn, root *ui.Node) (*ui.Node, error) {
	systray, err := root.DescendantWithTimeout(ctx, ui.FindParams{ClassName: "UnifiedSystemTray"}, uiTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to wait for UnifiedSystemTray")
	}
	defer systray.Release(ctx)

	if err := systray.LeftClick(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to click on UnifiedSystemTray")
	}

	tray, err := root.DescendantWithTimeout(ctx, ui.FindParams{ClassName: "SystemTrayContainer"}, uiTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to wait for SystemTrayContainer")
	}

	// Wait for the tray to show by waiting for the Night Light setting button.
	showNLSettings, err := tray.DescendantWithTimeout(ctx, ui.FindParams{Name: "Show Night Light settings"}, uiTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to wait for Show Night Light settings")
	}
	defer showNLSettings.Release(ctx)

	return tray, nil
}

func dismissSystemTray(ctx context.Context) error {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create keyboard writer")
	}
	defer kb.Close()

	if err := kb.Press(ctx, input.KEY_ESC); err != nil {
		return err
	}
	return nil
}

func nightLightSettingsWindow(ctx context.Context, tconn *chrome.Conn) (*ui.Node, error) {
	root, err := ui.Root(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get root UI node")
	}
	defer root.Release(ctx)

	tray, err := systemTray(ctx, tconn, root)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get system tray")
	}
	defer tray.Release(ctx)

	showNLSettings, err := tray.Descendant(ctx, ui.FindParams{Name: "Show Night Light settings"})
	if err != nil {
		return nil, errors.Wrap(err, "failed to wait for Show Night Light settings")
	}
	defer showNLSettings.Release(ctx)

	if err := showNLSettings.LeftClick(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to click to show Night Light settings")
	}

	settings, err := root.DescendantWithTimeout(ctx, ui.FindParams{ClassName: "BrowserFrame", Name: "Settings - Displays"}, uiTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find Settings window")
	}
	return settings, nil
}

func nightLightSchedulePopUp(ctx context.Context, tconn *chrome.Conn, settings *ui.Node) (*ui.Node, error) {
	schedule, err := settings.DescendantWithTimeout(ctx, ui.FindParams{Name: "Schedule"}, uiTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get schedule menu")
	}
	// TODO: UI relayout happens here, so sometimes clicking schedule miss. How to make deterministic?
	return schedule, nil
}

func closeWindow(ctx context.Context, tconn *chrome.Conn, window *ui.Node) error {
	defer window.Release(ctx)
	close, err := window.DescendantWithTimeout(ctx, ui.FindParams{ClassName: "FrameCaptionButton", Name: "Close"}, uiTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find window close button")
	}
	defer close.Release(ctx)
	if err := close.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click window close button")
	}
	return nil
}

func expandPopUp(ctx context.Context, tconn *chrome.Conn, popUp *ui.Node) error {
	if err := popUp.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click popUp")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := popUp.Refresh(ctx); err != nil {
			return err
		}
		if expanded, ok := popUp.State[ui.StateTypeExpanded]; !ok || !expanded {
			return errors.New("popUp not expanded")
		}
		return nil
	}, &testing.PollOptions{Timeout: uiTimeout}); err != nil {
		return errors.Wrap(err, "failed to wait for popUp to expand")
	}
	return nil
}

func releaseNodes(ctx context.Context, nodes []*ui.Node) {
	for _, node := range nodes {
		node.Release(ctx)
	}
}

func popUpOptions(ctx context.Context, tconn *chrome.Conn, popUp *ui.Node) ([]*ui.Node, error) {
	options, err := popUp.DescendantsWithTimeout(ctx, ui.FindParams{
		Role: ui.RoleTypeMenuListOption,
		State: map[ui.StateType]bool{
			ui.StateTypeFocusable: true,
		},
	}, uiTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get popUp options")
	}
	return options, nil
}

func nightLightSchedule(ctx context.Context, tconn *chrome.Conn) (string, error) {
	settings, err := nightLightSettingsWindow(ctx, tconn)
	if err != nil {
		return "", err
	}
	defer closeWindow(ctx, tconn, settings)

	schedule, err := nightLightSchedulePopUp(ctx, tconn, settings)
	if err != nil {
		return "", err
	}
	defer schedule.Release(ctx)

	var scheduleName string
	if err := schedule.Property(ctx, "value", &scheduleName); err != nil {
		return "", err
	}
	return scheduleName, nil
}

func setNightLightSchedule(ctx context.Context, tconn *chrome.Conn, scheduleName string) error {
	settings, err := nightLightSettingsWindow(ctx, tconn)
	if err != nil {
		return err
	}
	defer closeWindow(ctx, tconn, settings)

	schedule, err := nightLightSchedulePopUp(ctx, tconn, settings)
	if err != nil {
		return err
	}
	defer schedule.Release(ctx)

	if err := expandPopUp(ctx, tconn, schedule); err != nil {
		return err
	}

	options, err := popUpOptions(ctx, tconn, schedule)
	if err != nil {
		return err
	}
	defer releaseNodes(ctx, options)

	// NB: clicking on a menuListPopup menuListOption does not work because the
	// reported Location is incorrect, so use the keyboard instead.
	selectedIndex := -1
	targetIndex := -1
	for i, option := range options {
		if option.Name == scheduleName {
			targetIndex = i
		}
		var selected bool
		err := option.Property(ctx, "selected", &selected)
		if err != nil {
			return err
		}
		if selected {
			selectedIndex = i
		}
	}
	if targetIndex == -1 || selectedIndex == -1 {
		return errors.New("failed to find current and target schedule options")
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create keyboard writer")
	}
	defer kb.Close()

	for ; targetIndex < selectedIndex; targetIndex++ {
		if err := kb.Press(ctx, input.KEY_UP); err != nil {
			return errors.Wrap(err, "failed to use keyboard to select Night Light schedule")
		}
	}
	for ; targetIndex > selectedIndex; targetIndex-- {
		if err := kb.Press(ctx, input.KEY_DOWN); err != nil {
			return errors.Wrap(err, "failed to use keyboard to select Night Light schedule")
		}
	}
	if err := kb.Press(ctx, input.KEY_ENTER); err != nil {
		return err
	}
	return nil
	/*if dump, err := ui.RootDebugInfo(ctx, tconn); err == nil {
		testing.ContextLog(ctx, dump)
	}
	panic("done!")
	*/
}

func nightLightEnabled(ctx context.Context, tconn *chrome.Conn) (bool, error) {
	root, err := ui.Root(ctx, tconn)
	if err != nil {
		return false, errors.Wrap(err, "failed to get root UI node")
	}
	defer root.Release(ctx)

	tray, err := systemTray(ctx, tconn, root)
	if err != nil {
		return false, err
	}
	defer tray.Release(ctx)

	defer dismissSystemTray(ctx)

	toggleOn, err := tray.Descendant(ctx, ui.FindParams{Name: "Toggle Night Light. Night Light is off."})
	if err == nil {
		toggleOn.Release(ctx)
		return false, nil
	}
	toggleOff, err := tray.Descendant(ctx, ui.FindParams{Name: "Toggle Night Light. Night Light is on."})
	if err != nil {
		return false, errors.Wrap(err, "failed to find Night Light toggle")
	}
	toggleOff.Release(ctx)
	return true, nil
	/*
		if dump, err := ui.RootDebugInfo(ctx, tconn); err == nil {
			testing.ContextLog(ctx, dump)
		}
	panic("done!")*/
	//return enabled, nil
}

func setNightLightEnabled(ctx context.Context, tconn *chrome.Conn, enabled bool) error {
	root, err := ui.Root(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get root UI node")
	}
	defer root.Release(ctx)

	tray, err := systemTray(ctx, tconn, root)
	if err != nil {
		return err
	}
	defer tray.Release(ctx)

	defer dismissSystemTray(ctx)

	if enabled {
		toggleOn, err := tray.Descendant(ctx, ui.FindParams{Name: "Toggle Night Light. Night Light is off."})
		if err != nil {
			return err
		}
		defer toggleOn.Release(ctx)
		return toggleOn.LeftClick(ctx)
	}
	toggleOff, err := tray.Descendant(ctx, ui.FindParams{Name: "Toggle Night Light. Night Light is on."})
	if err != nil {
		return err
	}
	defer toggleOff.Release(ctx)
	return toggleOff.LeftClick(ctx)
}

// NightLightSchedule sets the Night Light schedule. Possible values are:
// "Never", "Sunset to Sunrise", "Custom"
func NightLightSchedule(ctx context.Context, tconn *chrome.Conn, scheduleName string) (CleanupCallback, error) {
	prevSchedule, err := nightLightSchedule(ctx, tconn)
	if err != nil {
		return nil, err
	}
	testing.ContextLogf(ctx, "Setting Night Light schedule to %q from %q", scheduleName, prevSchedule)
	if err := setNightLightSchedule(ctx, tconn, scheduleName); err != nil {
		return nil, err
	}

	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Resetting Night Light schedule to %q", prevSchedule)
		return setNightLightSchedule(ctx, tconn, prevSchedule)
	}, nil
}

// NightLightEnabled sets Night Light enabled or disabled.
func NightLightEnabled(ctx context.Context, tconn *chrome.Conn, enabled bool) (CleanupCallback, error) {
	prevEnabled, err := nightLightEnabled(ctx, tconn)
	if err != nil {
		return nil, err
	}
	testing.ContextLogf(ctx, "Setting Night Light enabled to %t from %t", enabled, prevEnabled)
	if err := setNightLightEnabled(ctx, tconn, enabled); err != nil {
		return nil, err
	}

	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Resetting Night Light enabled to %t", prevEnabled)
		return setNightLightEnabled(ctx, tconn, prevEnabled)
	}, nil
}

// DisableNightLight sets the Night Light schedule to 'Never' and disables Night
// Light.
func DisableNightLight(ctx context.Context, tconn *chrome.Conn) (CleanupCallback, error) {
	return Nested(ctx, func(s *Setup) error {
		s.Add(NightLightSchedule(ctx, tconn, "Never"))
		s.Add(NightLightEnabled(ctx, tconn, false))
		return nil
	})
}
