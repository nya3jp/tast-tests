// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"fmt"

	"chromiumos/tast/local/bundles/cros/apps/webstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

type appsParams struct {
	appName    string
	urlAppName string
	urlAppID   string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebstoreAppInstallUninstall,
		Desc:         "Chrome webstore app install uninstall",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{
			{
				Name: "google_photo",
				Val: appsParams{appName: "Google Photo",
					urlAppName: "save-image-to-google-phot",
					urlAppID:   "djakijfdccnmmnknpmphdkkjbjfenkne",
				},
				Fixture: "chromeLoggedIn",
			},
			{
				Name: "cut_the_rope",
				Val: appsParams{appName: "Cut The Rope",
					urlAppName: "cut-the-rope",
					urlAppID:   "dbcfgdjlnoeniakcjlefekcainimpida",
				},
				Fixture: "chromeLoggedIn",
			},
		},
	})
}

func WebstoreAppInstallUninstall(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(appsParams)
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	appURL := fmt.Sprintf("https://chrome.google.com/webstore/detail/%s/%s", testOpt.urlAppName, testOpt.urlAppID)

	// App Install parameters.
	app := webstore.App{Name: testOpt.appName,
		URL:           appURL,
		VerifyText:    "Remove from Chrome",
		AddRemoveText: "Add to Chrome",
		ConfirmText:   "Add extension",
	}

	s.Logf("Installing %q app", testOpt.appName)
	if err := webstore.UpgradeWebstoreApp(ctx, cr, tconn, app); err != nil {
		s.Fatal("Failed to install webapp: ", err)
	}

	// App Uninstall parameters.
	app = webstore.App{Name: testOpt.appName,
		URL:           appURL,
		VerifyText:    "Add to Chrome",
		AddRemoveText: "Remove from Chrome",
		ConfirmText:   "Remove",
	}

	s.Logf("Uninstalling %q app", testOpt.appName)
	if err := webstore.UpgradeWebstoreApp(ctx, cr, tconn, app); err != nil {
		s.Fatal("Failed to uninstall webapp: ", err)
	}
}
