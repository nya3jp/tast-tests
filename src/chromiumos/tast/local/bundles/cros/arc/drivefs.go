// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/storage"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/testing"
)

const (
	drivefsURI = "content://org.chromium.arc.volumeprovider/MyDrive/root/storage.txt"
)

// bootedWithGaiaLogin is a precondition similar to arc.Booted(). The only difference from arc.Booted() is
// that it will GAIA login with the arc.Drivefs credentials.
var bootedWithGaiaLogin = arc.NewPrecondition("bootedWithGaiaLogin", false, drivefsGaia)

var drivefsGaia = &arc.GaiaVars{
	UserVar: "arc.Drivefs.user",
	PassVar: "arc.Drivefs.password",
}

func init() {
	testing.AddTest(&testing.Test{
		Func: Drivefs,
		Desc: "Android app can read files on Drive FS (Google Drive) via FilesApp",
		Contacts: []string{
			"cherieccy@google.com",
			"arc-storage@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          bootedWithGaiaLogin,
		Vars:         []string{"arc.Drivefs.user", "arc.Drivefs.password"},
		Params: []testing.Param{
			{
				Val: []storage.Expectation{
					{storage.ActionID, storage.ExpectedAction},
					{storage.FileContentID, storage.ExpectedFileContent}},
				ExtraSoftwareDeps: []string{"android_p"},
			}, {
				Name: "vm",
				Val: []storage.Expectation{
					{storage.ActionID, storage.ExpectedAction},
					{storage.URIID, drivefsURI},
					{storage.FileContentID, storage.ExpectedFileContent}},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
		},
	})
}

func Drivefs(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome
	expectations := s.Param().([]storage.Expectation)

	mountPath, err := drivefs.WaitForDriveFs(ctx, cr.User())
	if err != nil {
		s.Fatal("Failed waiting for DriveFS to start: ", err)
	}
	drivefsRoot := path.Join(mountPath, "root")
	dir := storage.Directory{Path: drivefsRoot, Name: "Google Drive", Title: "Files - My Drive"}

	storage.TestOpenWithAndroidApp(ctx, s, a, cr, dir, expectations)
}
