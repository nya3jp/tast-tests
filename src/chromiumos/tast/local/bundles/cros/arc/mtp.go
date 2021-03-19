// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/storage"
	"chromiumos/tast/testing"
)

// mtpURI contains vendor / model id of the Android device under test.
const (
	mtpURI = "content://org.chromium.arc.chromecontentprovider/externalfile%3Afileman-mtp-mtp%253AVendorModelVolumeStorage%253A6353%253A20193%253A%253A65537%2FDownload%2F"
)

// arc.Mtp / arc.Mtp.vm tast tests depend on the use of actual Android device in the lab.
// As part of the test, a file will be pushed and read from it. Therefore, these tests have
// the following constraints:
// 1. It can only be run on a special lab setup.
// 2. The device folder names etc being used are hard-coded for the setup.

func init() {
	testing.AddTest(&testing.Test{
		Func: Mtp,
		Desc: "ARC++/ARCVM Android app can read files on external Android device (with MTP) via FilesApp",
		Contacts: []string{
			"cherieccy@google.com",
			"arc-storage@google.com",
		},
		// TODO(b/179736612): Enable the test when the lab setup is ready.
		//Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Fixture:      "arcBooted",
		Params: []testing.Param{
			{
				ExtraSoftwareDeps: []string{"android_p"},
			}, {
				Name:              "vm",
				ExtraSoftwareDeps: []string{"android_vm"},
			},
		},
	})
}

func Mtp(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome

	config := storage.TestConfig{DirName: "Nexus/Pixel (MTP)", DirTitle: "Files - Nexus/Pixel (MTP)",
		SubDirectories: []string{"Download"}, FileName: "storage.txt"}
	expectations := []storage.Expectation{
		{LabelID: storage.ActionID, Value: storage.ExpectedAction},
		{LabelID: storage.URIID, Value: mtpURI + config.FileName},
		{LabelID: storage.FileContentID, Value: storage.ExpectedFileContent}}

	storage.TestOpenWithAndroidApp(ctx, s, a, cr, config, expectations)
}
