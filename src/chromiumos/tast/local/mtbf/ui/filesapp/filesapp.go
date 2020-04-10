// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filesapp

import (
	"context"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/ui/filesapp"
)

// Launch launches the Files App and returns it.
// A mtbf error is returned if the app fails to launch.
func Launch(ctx context.Context, tconn *chrome.Conn) (*filesapp.FilesApp, error) {
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return files, mtbferrors.New(mtbferrors.ChromeOpenFileApps, err)
	}
	return files, err
}
