// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/bundles/cros/graphics/webstore"
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
		Desc:         "Verifies chrome webstore app install uninstall",
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
				Timeout: 10 * time.Minute,
			},
			{
				Name: "cut_the_rope",
				Val: appsParams{appName: "Cut The Rope",
					urlAppName: "cut-the-rope",
					urlAppID:   "dbcfgdjlnoeniakcjlefekcainimpida",
				},
				Timeout: 10 * time.Minute,
			},
		},
	})
}

func WebstoreAppInstallUninstall(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(appsParams)
	cr, err := chrome.New(ctx)

	if err != nil {
		s.Fatal("Failed to start chrome: ", err)
	}

	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)

	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	var appURL = fmt.Sprintf("https://chrome.google.com/webstore/detail/%s/%s", testOpt.urlAppName, testOpt.urlAppID)

	// App Install parameters.
	app := webstore.App{Name: testOpt.appName,
		URL: appURL, InstalledTxt: "Remove from Chrome",
		AddTxt:     "Add to Chrome",
		ConfirmTxt: "Add extension",
	}

	s.Logf("Installing %s app", testOpt.appName)

	if err := webstore.InstallWebstoreApp(ctx, cr, tconn, app); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	// App Uninstall parameters.
	app = webstore.App{Name: testOpt.appName,
		URL: appURL, InstalledTxt: "Add to Chrome",
		AddTxt:     "Remove from Chrome",
		ConfirmTxt: "Remove",
	}

	s.Logf("Uninstalling %s app", testOpt.appName)

	if err := webstore.InstallWebstoreApp(ctx, cr, tconn, app); err != nil {
		s.Fatal("Failed to un-install app: ", err)
	}
}
