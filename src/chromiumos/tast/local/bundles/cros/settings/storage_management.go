// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package settings

import (
	"context"
	"time"
	"regexp"

  "chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	//"chromiumos/tast/local/chrome/uiauto/ossettings"
	//"chromiumos/tast/local/chrome/uiauto/role"
	//"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: StorageManagement,
		Desc: "Validates the information presented in the storage management page",
		Contacts: []string{
			"chromeos-storage@google.com",
      "chromeos-files-syd@google.com",
    },
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

// StorageManagement queries the values from the storage management page and
// validates the values against on-disk state (via data from spaced).
func StorageManagement(ctx context.Context, s *testing.State) {
  cr := s.FixtValue().(*chrome.Chrome)

  tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

  _, err = apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/storage")
	if err != nil {
		s.Fatal("Failed to launch Settings: ", err)
	}

	testing.Sleep(ctx, 5*time.Second)

  uia := uiauto.New(tconn)
	text, _ := uia.Info(ctx, nodewith.NameRegex("In Use"))
	s.Fatal("hello ", text.Description)

}