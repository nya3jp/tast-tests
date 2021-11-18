// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package imesettings

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ime"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/useractions"
)

// SetGlideTyping returns an user action to change 'Glide suggestions' setting.
func SetGlideTyping(uc *useractions.UserContext, im ime.InputMethod, isEnabled bool) *useractions.UserAction {
	actionName := "enable glide typing in IME setting"
	if !isEnabled {
		actionName = "disable glide typing in IME setting"
	}

	action := func(ctx context.Context) error {
		setting, err := LaunchAtInputsSettingsPage(ctx, uc.TestAPIConn(), uc.Chrome())
		if err != nil {
			return errors.Wrap(err, "failed to launch input settings")
		}
		return uiauto.Combine("change glide typing setting",
			setting.OpenInputMethodSetting(uc.TestAPIConn(), im),
			setting.ToggleGlideTyping(uc.Chrome(), isEnabled),
			setting.Close,
		)(ctx)
	}

	return useractions.NewUserAction(actionName, action, uc, &useractions.UserActionCfg{
		ActionTags: []useractions.ActionTag{
			useractions.ActionTagEssentialInputs,
			useractions.ActionTagIMESettings,
			useractions.ActionTagGlideTyping,
		},
	})
}
