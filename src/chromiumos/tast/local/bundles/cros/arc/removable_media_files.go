// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/removablemedia"
	"chromiumos/tast/local/bundles/cros/arc/storage"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RemovableMediaFiles,
		Desc:         "Verifies that Android apps can read and write files inside removable media from the Files app",
		Contacts:     []string{"youkichihosoi@google.com", "arc-storage@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Data:         []string{"capybara.jpg"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 6 * time.Minute,
	})
}
func RemovableMediaFiles(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome

	const (
		diskName  = "MyDisk"
		imageSize = 64 * 1024 * 1024
	)

	mountPath, cleanupFunc, err := removablemedia.SetUpRemovableMediaForTesting(ctx, diskName, imageSize)
	if err != nil {
		s.Fatal("Failed to set up removable media for testing: ", err)
	}
	defer cleanupFunc(ctx)

	if err := storage.WaitForARCVolumeMount(ctx, a, removablemedia.FakeUUID); err != nil {
		s.Fatal("Failed to wait for the volume to be mounted in ARC: ", err)
	}

	testARCToCrosRemovableMedia(ctx, s, a, mountPath)

	testCrosToARCRemovableMedia(ctx, s, a, cr, diskName, mountPath)
}

// testARCToCrosRemovableMedia checks whether a file put in a removable media
// volume on the Android sie appears in the corresponding directory on the
// Chrome OS side.
func testARCToCrosRemovableMedia(ctx context.Context, s *testing.State, a *arc.ARC, mountPath string) {
	const (
		filename    = "capybara.jpg"
		androidPath = "/storage/" + removablemedia.FakeUUID + "/" + filename
	)
	crosPath := mountPath + "/" + filename

	testing.ContextLog(ctx, "Testing Android -> CrOS")

	if err := storage.TestPushToARCAndReadFromCros(ctx, a, s.DataPath(filename), androidPath, crosPath); err != nil {
		s.Fatal("Android -> CrOS failed: ", err)
	}
}

// testCrosToARCRemovableMedia checks whether a file put in a removable media
// device on the Chrome OS side can be opened with Android apps from the Files
// app.
func testCrosToARCRemovableMedia(ctx context.Context, s *testing.State, a *arc.ARC, cr *chrome.Chrome, diskName, mountPath string) {
	config := storage.TestConfig{DirPath: mountPath, DirName: diskName, DirTitle: "Files - " + diskName,
		CreateTestFile: true, WriteFileContentWithApp: true, FileName: "storage.txt"}
	testFileURI := storage.VolumeProviderContentURIPrefix + removablemedia.FakeUUID + "/" + config.FileName

	testing.ContextLog(ctx, "Testing CrOS -> Android")

	expectations := []storage.Expectation{
		{LabelID: storage.ActionID, Value: storage.ExpectedAction},
		{LabelID: storage.URIID, Value: testFileURI},
		{LabelID: storage.FileContentID, Value: storage.ExpectedFileContent}}

	storage.TestOpenWithAndroidApp(ctx, s, a, cr, config, expectations)
}
