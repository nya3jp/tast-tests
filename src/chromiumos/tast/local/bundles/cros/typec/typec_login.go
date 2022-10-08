// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/typecutils"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TypecLogin,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Processes login and enables data peripheral access setting (when mode set to complete) to enable TBT/USB4",
		Contacts:     []string{"rajat.khandelwal@intel.com"},
		Attr:         []string{"group:typec", "typec_informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		Data:         []string{"testcert.p12"},
		Params: []testing.Param{
			// To login and change mode without enabling data peripheral access
			{
				Name:      "normal",
				Val:       false,
			},
			// To login and enable data peripheral access
			{
				Name:      "complete",
				Val:       true,
			}},
	})
}

// TypecLogin does the following:
// If normal, login happens with only changing mode, if required.
// If complete, data peripheral access is also enabled.
//
func TypecLogin(ctx context.Context, s *testing.State) {
	// Get to the Chrome login screen.
	cr, err := chrome.New(ctx,
		chrome.DeferLogin(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))
	if err != nil {
		s.Fatal("Failed to start Chrome at login screen: ", err)
	}
	defer cr.Close(ctx)

	peripheralDataEnableReq := s.Param().(bool)
	if peripheralDataEnableReq == true {
		if err = typecutils.EnablePeripheralDataAccess(ctx, s.DataPath("testcert.p12")); err != nil {
			s.Fatal("Failed to enable peripheral data access setting: ", err)
		}
	}

	if err := cr.ContinueLogin(ctx); err != nil {
		s.Fatal("Failed to login: ", err)
	}
}
