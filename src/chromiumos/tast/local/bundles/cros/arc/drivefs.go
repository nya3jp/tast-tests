// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/storage"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/testing"
)

const (
	drivefsURI = "content://org.chromium.arc.volumeprovider/MyDrive/root/storage.txt"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Drivefs,
		Desc: "Android app can read files on Drive FS (Google Drive) via FilesApp",
		Contacts: []string{
			"cherieccy@google.com",
			"arc-storage@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "chrome_internal", "drivefs"},
		Timeout:      5 * time.Minute,
		Vars:         []string{"arc.Drivefs.user", "arc.Drivefs.password"},
		Params: []testing.Param{
			{
				Val: []storage.Expectation{
					{LabelID: storage.ActionID, Value: storage.ExpectedAction},
					{LabelID: storage.FileContentID, Value: storage.ExpectedFileContent}},
				ExtraSoftwareDeps: []string{"android_p"},
			}, {
				Name: "vm",
				Val: []storage.Expectation{
					{LabelID: storage.ActionID, Value: storage.ExpectedAction},
					{LabelID: storage.URIID, Value: drivefsURI},
					{LabelID: storage.FileContentID, Value: storage.ExpectedFileContent}},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
		},
	})
}

func Drivefs(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(
		ctx,
		chrome.ARCEnabled(),
		chrome.Auth(s.RequiredVar("arc.Drivefs.user"), s.RequiredVar("arc.Drivefs.password"), ""),
		chrome.GAIALogin(),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	expectations := s.Param().([]storage.Expectation)

	mountPath, err := drivefs.WaitForDriveFs(ctx, cr.User())
	if err != nil {
		s.Fatal("Failed waiting for DriveFS to start: ", err)
	}
	drivefsRoot := path.Join(mountPath, "root")

	dir := storage.Directory{Path: drivefsRoot, Name: "Google Drive", Title: "Files - My Drive", CheckFileType: true}

	storage.TestOpenWithAndroidApp(ctx, s, a, cr, dir, expectations)
}
