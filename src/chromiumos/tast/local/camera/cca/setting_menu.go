// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cca provides utilities to interact with Chrome Camera App.
package cca

import (
	"context"
	"fmt"

	"chromiumos/tast/errors"
)

// SettingMenu is the setting menu in CCA.
type SettingMenu struct {
	view   string
	openUI *UIComponent
}

var (
	// MainMenu is the main setting menu.
	MainMenu = &SettingMenu{"view-settings", &SettingsButton}
	// GridTypeMenu is the grid settings menu.
	GridTypeMenu = &SettingMenu{"view-grid-settings", &GridTypeSettingsButton}
	// TimerMenu is the timer settings menu.
	TimerMenu = &SettingMenu{"view-timer-settings", &TimerSettingsButton}
	// ResolutionMenu is the resolution settings menu.
	ResolutionMenu = &SettingMenu{"view-resolution-settings", &ResolutionSettingButton}
	// ExpertMenu is the expert settings menu.
	ExpertMenu = &SettingMenu{"view-expert-settings", &ExpertModeButton}
)

// Open opens the menu.
func (menu *SettingMenu) Open(ctx context.Context, app *App) error {
	if err := app.Click(ctx, *menu.openUI); err != nil {
		return err
	}
	active, err := app.GetState(ctx, menu.view)
	if err != nil {
		return errors.Wrap(err, "failed to get view open state")
	}
	if !active {
		return errors.Errorf("view %q is not openned", menu.view)
	}
	return nil
}

// Close closes the menu.
func (menu *SettingMenu) Close(ctx context.Context, app *App) error {
	name := fmt.Sprintf("%s back button", menu.view)
	selector := fmt.Sprintf("#%s .menu-header button", menu.view)
	closeUI := UIComponent{name, []string{selector}}
	return app.Click(ctx, closeUI)
}
