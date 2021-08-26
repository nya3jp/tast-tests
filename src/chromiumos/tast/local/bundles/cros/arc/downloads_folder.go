// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/storage"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

const (
	downloadURI = "content://org.chromium.arc.volumeprovider/download/storage.txt"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DownloadsFolder,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Android app can read files on Downloads folder via FilesApp",
		Contacts: []string{
			"youkichihosoi@chromium.org",
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

	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}
	config := storage.TestConfig{DirPath: downloadsPath, DirName: "Downloads",
		DirTitle: "Files - Downloads", CreateTestFile: true, FileName: "storage.txt"}

	// In ARCVM, Downloads integration depends on MyFiles mount.
	if err := arc.WaitForARCMyFilesVolumeMountIfARCVMEnabled(ctx, a); err != nil {
		s.Fatal("Failed to wait for MyFiles to be mounted in ARC: ", err)
	}

	storage.TestOpenWithAndroidApp(ctx, s, a, cr, d, config, expectations)
}
