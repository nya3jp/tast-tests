// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"path"
	"strings"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/storage"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/testing"
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
		Timeout:      4 * time.Minute,
		Vars:         []string{"arc.Drivefs.user", "arc.Drivefs.password"},
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

func Drivefs(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(
		ctx,
		chrome.ARCEnabled(),
		chrome.GAIALogin(chrome.Creds{
			User: s.RequiredVar("arc.Drivefs.user"),
			Pass: s.RequiredVar("arc.Drivefs.password"),
		}),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	vmEnabled, err := arc.VMEnabled()
	if err != nil {
		s.Fatal("Failed to check if VM is enabled: ", err)
	}

	mountPath, err := drivefs.WaitForDriveFs(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed waiting for DriveFS to start: ", err)
	}
	drivefsRoot := path.Join(mountPath, "root")

	config := storage.TestConfig{DirPath: drivefsRoot, DirName: "Google Drive", DirTitle: "Files - My Drive",
		CreateTestFile: true, CheckFileType: true, FileName: randFileName()}
	expectations := []storage.Expectation{
		{LabelID: storage.ActionID, Value: storage.ExpectedAction},
		{LabelID: storage.URIID, Value: constructDriveFSURI(vmEnabled, drivefsRoot, config.FileName)},
		{LabelID: storage.FileContentID, Value: storage.ExpectedFileContent}}

	storage.TestOpenWithAndroidApp(ctx, s, a, cr, config, expectations)
}

// randFileName generates a randomized test file name to avoid race condition among concurrently running tests.
func randFileName() string {
	return fmt.Sprintf("storage_%d.txt", rand.Intn(1000000000))
}

// constructDriveFSURI constructs a Drive FS URI.
func constructDriveFSURI(vmEnabled bool, drivefsRoot, file string) string {
	if vmEnabled {
		return "content://org.chromium.arc.volumeprovider/MyDrive/root/" + file
	}
	subPath := strings.ReplaceAll(drivefsRoot, "/media/fuse/", "") + "/"
	return "content://org.chromium.arc.chromecontentprovider/externalfile%3A" + url.PathEscape(subPath) + file
}
