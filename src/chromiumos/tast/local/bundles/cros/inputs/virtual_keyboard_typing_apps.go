// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/vkb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardTypingApps,
		Desc:         "Checks that the virtual keyboard works in apps",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name:              "stable",
			Pre:               pre.VKEnabledTablet,
			ExtraHardwareDeps: pre.InputsStableModels,
			ExtraAttr:         []string{"group:input-tools-upstream"},
		}, {
			Name:              "unstable",
			Pre:               pre.VKEnabledTablet,
			ExtraHardwareDeps: pre.InputsUnstableModels,
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "exp",
			Pre:               pre.VKEnabledTabletExp,
			ExtraSoftwareDeps: []string{"gboard_decoder"},
			ExtraAttr:         []string{"informational", "group:input-tools-upstream"},
		}}})
}

func VirtualKeyboardTypingApps(ctx context.Context, s *testing.State) {
	// typingKeys indicates a key series that tapped on virtual keyboard.
	// Input string should start with lower case letter because VK layout is not auto-capitalized in the settings search bar.
	const typingKeys = "language"

	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	searchInputElement := nodewith.Role(role.SearchBox).Name("Search settings")
	app := apps.Settings
	if err := uiauto.Combine("type in an app",
		apps.LaunchAction(tconn, app.ID),
		ash.WaitForAppAction(tconn, app.ID),
		uiauto.WaitForLocationChangeCompletedAction(tconn),
		vkb.ClickUntilVKShownAction(tconn, searchInputElement),
		vkb.WaitForVKReadyAction(tconn, cr),
		vkb.TapKeysAction(tconn, strings.Split(typingKeys, "")),
		// Hide virtual keyboard to submit candidate
		vkb.HideVirtualKeyboardAction(tconn),
		uiauto.New(tconn).WithTimeout(10*time.Second).Poll(func(ctx context.Context) error {
			info, err := uiauto.New(tconn).Info(ctx, searchInputElement)
			if err != nil {
				return errors.Wrap(err, "failed to get the node's location")
			}

			if info.Value != typingKeys {
				return errors.Errorf("failed to input with virtual keyboard. Got: %s; Want: %s", info.Value, typingKeys)
			}
			return nil
		}),
	)(ctx); err != nil {
		s.Fatal("Failed to type in an app: ", err)
	}
}
