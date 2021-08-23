// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/storage"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
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
			"arc-storage@google.com",
			"cros-arc-te@google.com",
		},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 4 * time.Minute,
		Fixture: "arcBooted",
	})
}

func DownloadsFolder(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome
	d := s.FixtValue().(*arc.PreData).UIDevice
	expectations := []storage.Expectation{
		{LabelID: storage.ActionID, Value: storage.ExpectedAction},
		{LabelID: storage.URIID, Value: downloadURI},
		{LabelID: storage.FileContentID, Value: storage.ExpectedFileContent}}

	config := storage.TestConfig{DirPath: filesapp.DownloadPath, DirName: "Downloads",
		DirTitle: "Files - Downloads", CreateTestFile: true, FileName: "storage.txt"}

	storage.TestOpenWithAndroidApp(ctx, s, a, cr, d, config, expectations)
}
