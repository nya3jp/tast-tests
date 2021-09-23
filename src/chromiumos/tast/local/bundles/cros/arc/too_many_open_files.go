// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

type arcTooManyOpenFilesParams struct {
	crosTargetDir    func(ctx context.Context, user, dirname string) (string, error)
	androidTargetDir func(dirname string) string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         TooManyOpenFiles,
		Desc:         "Reproduce the \"Too many open files\" error on virtio-fs",
		Contacts:     []string{"youkichihosoi@chromium.org", "arc-storage@google.com"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Fixture:      "arcBooted",
		Params: []testing.Param{{
			Name: "sdcard",
			Val: arcTooManyOpenFilesParams{
				crosTargetDir:    crosSdcardTargetDir,
				androidTargetDir: androidSdcardTargetDir,
			},
		}, {
			Name: "myfiles",
			Val: arcTooManyOpenFilesParams{
				crosTargetDir:    crosMyFilesTargetDir,
				androidTargetDir: androidMyFilesTargetDir,
			},
		}},
	})
}

func crosSdcardTargetDir(ctx context.Context, user, dirname string) (string, error) {
	androidDataDir, err := arc.AndroidDataDir(user)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get android-data path for user %v", user)
	}
	documentsDir := filepath.Join(androidDataDir, "data", "media", "0", "Documents")
	return filepath.Join(documentsDir, dirname), nil
}

func androidSdcardTargetDir(dirname string) string {
	return filepath.Join("/storage", "emulated", "0", "Documents", dirname)
}

func crosMyFilesTargetDir(ctx context.Context, user, dirname string) (string, error) {
	cryptohomeUserPath, err := cryptohome.UserPath(ctx, user)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get cryptohome user path for user %v", user)
	}
	myfilesDir := filepath.Join(cryptohomeUserPath, "MyFiles")
	return filepath.Join(myfilesDir, dirname), nil
}

func androidMyFilesTargetDir(dirname string) string {
	return filepath.Join("/storage", arc.MyFilesUUID, dirname)
}

func TooManyOpenFiles(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome
	params := s.Param().(arcTooManyOpenFilesParams)

	const (
		numberOfFiles = 16384
		targetDirName = "ArcTooManyOpenFiles"
		apkName       = "ArcTooManyOpenFilesTest.apk"
		packageName   = "org.chromium.arc.testapp.toomanyopenfiles"
		targetPathKey = "target_path"
	)

	crosTargetDir, err := params.crosTargetDir(ctx, cr.NormalizedUser(), targetDirName)
	if err != nil {
		s.Fatal("Failed to get the target directory path: ", err)
	}

	if err := os.MkdirAll(crosTargetDir, 0755); err != nil {
		s.Fatalf("Failed to create the target directory %v: %v", crosTargetDir, err)
	}
	// defer os.RemoveAll(crosTargetDir)

	for i := 0; i < numberOfFiles; i++ {
		dst := filepath.Join(crosTargetDir, fmt.Sprintf("file_%d.txt", i))
		file, err := os.Create(dst)
		if err != nil {
			s.Fatalf("Failed to create file %v: %v", dst, err)
		}
		file.Close()
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	if err := a.Install(ctx, arc.APKPath(apkName), adb.InstallOptionGrantPermissions); err != nil {
		s.Fatalf("Failed to install %v: %v", apkName, err)
	}

	if err := a.Command(ctx, "appops", "set", packageName, "MANAGE_EXTERNAL_STORAGE", "allow").Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to grant MANAGE_EXTERNAL_STORAGE to %v: %v", packageName, err)
	}

	act, err := arc.NewActivity(a, packageName, ".MainActivity")
	if err != nil {
		s.Fatalf("Failed to create the main activity for %v: %v", packageName, err)
	}
	defer act.Close()

	androidTargetDir := params.androidTargetDir(targetDirName)
	startCommandPrefixes := []string{"-S", "-W", "-n"}
	startCommandSuffixes := []string{"--es", targetPathKey, androidTargetDir}

	if err := act.StartWithArgs(ctx, tconn, startCommandPrefixes, startCommandSuffixes); err != nil {
		s.Fatalf("Failed to start the main activity of %v: %v", packageName, err)
	}
}
