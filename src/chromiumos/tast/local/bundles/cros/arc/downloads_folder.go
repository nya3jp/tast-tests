// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/storage"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/testing"
)

const (
	downloadURI = "content://org.chromium.arc.file_system.fileprovider/download/storage.txt"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DownloadsFolder,
		Desc: "Android app can read files on Downloads folder via FilesApp",
		Contacts: []string{
			"cherieccy@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          arc.Booted(),
		Params: []testing.Param{
			{
				Val: []storage.AppLabel{
					{storage.ActionID, storage.ExpectedAction},
					{storage.URIID, downloadURI},
					{storage.FileContentID, storage.ExpectedFileContent}},
				ExtraSoftwareDeps: []string{"android_p"},
			}, {
				Name: "vm",
				Val: []storage.AppLabel{
					{storage.ActionID, storage.ExpectedAction},
					{storage.URIID, downloadURI},
					{storage.FileContentID, storage.ExpectedFileContent}},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
		},
	})
}

func DownloadsFolder(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome
	appLabels := s.Param().([]storage.AppLabel)

	dir := storage.Directory{Path: filesapp.DownloadPath, Name: "Downloads", Title: "Files - Downloads"}

	storage.TestOpenWithAndroidApp(ctx, s, a, cr, dir, appLabels)
}
