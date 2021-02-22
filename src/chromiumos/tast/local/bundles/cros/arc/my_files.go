// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/storage"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

// myFilesUUID is the UUID of the MyFiles volume inside ARC. It is defined in
// Chromium's components/arc/volume_mounter/arc_volume_mounter_bridge.cc.
const myFilesUUID = "0000000000000000000000000000CAFEF00D2019"

func init() {
	testing.AddTest(&testing.Test{
		Func: MyFiles,
		Desc: "Checks whether the MyFiles directory is properly shared from Chrome OS to ARC",
		Contacts: []string{
			"youkichihosoi@chromium.org", "arc-eng@google.com",
		},
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
		Timeout: 5 * time.Minute,
	})
}

func MyFiles(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome

	cryptohomeUserPath, err := cryptohome.UserPath(ctx, cr.User())
	if err != nil {
		s.Fatalf("Failed to get the cryptohome user path for %s: %v", cr.User(), err)
	}
	myFilesPath := cryptohomeUserPath + "/MyFiles"

	testArcToCros(ctx, s, a, myFilesPath)
	testCrosToArc(ctx, s, a, cr, myFilesPath)
}

// testArcToCros checks whether a file put in the Android MyFiles directory
// appears in the Chrome OS MyFiles directory.
func testArcToCros(ctx context.Context, s *testing.State, a *arc.ARC, myFilesPath string) {
	const (
		filename    = "capybara.jpg"
		androidPath = "/storage/" + myFilesUUID + "/" + filename
	)
	crosPath := myFilesPath + "/" + filename

	if err := storage.TestPushToArcAndReadFromCros(ctx, a, s.DataPath(filename), androidPath, crosPath); err != nil {
		s.Fatal("Android -> CrOS failed: ", err)
	}
}

// testCrosToArc checks whether a file put in the Chrome OS MyFiles directory
// can be read by Android apps.
func testCrosToArc(ctx context.Context, s *testing.State, a *arc.ARC, cr *chrome.Chrome, myFilesPath string) {
	const testFileURI = "content://org.chromium.arc.volumeprovider/" + myFilesUUID + "/" + storage.TestFile

	expectations := []storage.Expectation{
		{LabelID: storage.ActionID, Value: storage.ExpectedAction},
		{LabelID: storage.URIID, Value: testFileURI},
		{LabelID: storage.FileContentID, Value: storage.ExpectedFileContent}}

	dir := storage.Directory{Path: myFilesPath, Name: "My files", Title: "Files - My files"}

	storage.TestOpenWithAndroidApp(ctx, s, a, cr, dir, expectations)
}
