// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"fmt"

	"chromiumos/tast/local/chrome"
)

// App represents a ChromeOS app
type App struct {
	ID   string
	Name string
}

// Chrome has details about the Chrome app
var Chrome = App{
	ID:   "mgndgikekgjfcpckkfioiadnlibdjbkf",
	Name: "Google Chrome",
}

// Files has details about the Files app
var Files = App{
	ID:   "hhaomjibdihmijegdhdafkllkbggdgoj",
	Name: "Files",
}

// WallpaperPicker has details about the Wallpaper Picker app
var WallpaperPicker = App{
	ID:   "obklkkbkpaoaejdabbfldmcfplpdgolj",
	Name: "Wallpaper Picker",
}

// LaunchApp launches an app
// appID is the ID of the app to launch
func LaunchApp(ctx context.Context, tconn *chrome.Conn, appID string) error {
	launchQuery := fmt.Sprintf("tast.promisify(chrome.autotestPrivate.launchApp)(%q)", appID)
	return tconn.EvalPromise(ctx, launchQuery, nil)
}
