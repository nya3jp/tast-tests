// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/storage"
	"chromiumos/tast/local/chrome/mtp"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

// mtpURIPrefix is the expected prefix of the Content URI for the Android
// device under test. The full URI would contain the device's serial number,
// which would be different for different devices.
const (
	mtpURIPrefix = "content://org.chromium.arc.chromecontentprovider/externalfile%3Afileman-mtp-mtp"
)

// arc.Mtp / arc.Mtp.vm tast tests depend on the use of actual Android device in the lab.
// As part of the test, a file will be pushed and read from it. Therefore, these tests have
// the following constraints:
// 1. It can only be run on a special lab setup.
// 2. The device folder names etc being used are hard-coded for the setup.

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTP,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "ARC++/ARCVM Android app can read files on external Android device (with MTP) via FilesApp",
		Contacts: []string{
			"youkichihosoi@chromium.org",
			"arc-storage@google.com",
			"cros-arc-te@google.com",
		},
		Attr:         []string{"group:mtp"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Fixture:      "mtpWithAndroid",
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

func MTP(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*mtp.FixtData).Chrome
	tconn := s.FixtValue().(*mtp.FixtData).TestConn

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	//TODO(b/187740535): Investigate and reserve time for cleanup.
	defer a.Close(ctx)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	config := storage.TestConfig{DirName: "Nexus/Pixel (MTP+ADB)", DirTitle: "Files - Nexus/Pixel (MTP+ADB)",
		SubDirectories: []string{"Download"}, FileName: "storage.txt"}
	expectations := []storage.Expectation{
		{LabelID: storage.ActionID, Value: storage.ExpectedAction},
		{LabelID: storage.URIID, Predicate: func(actual string) bool {
			return strings.HasPrefix(actual, mtpURIPrefix) &&
				strings.HasSuffix(actual, "%2FDownload%2Fstorage.txt")
		}},
		{LabelID: storage.FileContentID, Value: storage.ExpectedFileContent}}

	storage.TestOpenWithAndroidApp(ctx, s, a, cr, d, config, expectations)
}
