// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         QuotaProjectID,
		Desc:         "Verifies that Android's quota project ID setting logic works",
		Contacts:     []string{"hashimoto@chromium.org", "arcvm-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Fixture:      "arcBooted",
	})
}

func getQuotaProjectID(ctx context.Context, path string) (int64, error) {
	// Output looks like:
	// " 1003 ---------E----e----- /home/root/<hash>/android-data/data/media/0/Pictures/test.png"
	out, err := testexec.CommandContext(ctx, "lsattr", "-p", path).Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.Split(strings.TrimSpace(string(out)), " ")[0], 10, 64)
}

func QuotaProjectID(ctx context.Context, s *testing.State) {
	const androidUIDOffset = 655360
	// This number comes from Android's android_filesystem_config.h.
	const aidAppStart = 10000
	// These numbers come from Android's android_projectid_config.h.
	const (
		projectIDExtMediaImage = 1003
		projectIDExtDataStart  = 20000
	)

	const (
		apkName      = "ArcQuotaProjectIdTest.apk"
		pkgName      = "org.chromium.arc.testapp.quotaprojectid"
		activityName = "org.chromium.arc.testapp.quotaprojectid.MainActivity"
	)

	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	s.Log("Installing " + apkName)
	if err := a.Install(ctx, arc.APKPath(apkName)); err != nil {
		s.Fatal("Failed to install the APK: ", err)
	}

	act, err := arc.NewActivity(a, pkgName, activityName)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	s.Log("Starting MainActivity")
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start MainActivity: ", err)
	}
	defer act.Stop(ctx, tconn)

	// Check the project ID of the package data directory.
	pkgDataDir, err := arc.PkgDataDir(cr.NormalizedUser(), pkgName)
	if err != nil {
		s.Fatal("Failed to get package data dir: ", err)
	}
	fileInfo, err := os.Stat(pkgDataDir)
	if err != nil {
		s.Fatal("Failed to stat the package data dir: ", err)
	}
	stat, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		s.Fatal("Failed to get the stat of the package data dir")
	}
	pkgProjectID := int64(stat.Uid - androidUIDOffset - aidAppStart + projectIDExtDataStart)
	projectID, err := getQuotaProjectID(ctx, pkgDataDir)
	if err != nil {
		s.Fatal("Failed to get the project ID: ", err)
	}
	if projectID != pkgProjectID {
		s.Errorf("Unexpected project ID: %d, expected %d", projectID, pkgProjectID)
	}

	// Check the project ID of the file in the external files dir.
	externalFilesDirPath := filepath.Join(pkgDataDir, "files/Pictures/test.png")
	projectID, err = getQuotaProjectID(ctx, externalFilesDirPath)
	if err != nil {
		s.Fatal("Failed to get the project ID: ", err)
	}
	if projectID != pkgProjectID {
		s.Errorf("Unexpected project ID: %d, expected %d", projectID, pkgProjectID)
	}

	// Check the project ID of the file in the primary external volume.
	androidDataDir, err := arc.AndroidDataDir(cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get Android data dir: ", err)
	}
	primaryExternalVolumePath := filepath.Join(androidDataDir, "data/media/0/Pictures/test.png")
	projectID, err = getQuotaProjectID(ctx, primaryExternalVolumePath)
	if err != nil {
		s.Fatal("Failed to get the project ID: ", err)
	}
	if projectID != projectIDExtMediaImage {
		s.Errorf("Unexpected project ID: %d, expected %d",
			projectID, projectIDExtMediaImage)
	}

	// Check the project ID of the file in the Downloads directory.
	userPath, err := cryptohome.UserPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get the cryptohome user directory: ", err)
	}
	downloadsDirPath := filepath.Join(userPath, "Downloads")
	projectID, err = getQuotaProjectID(ctx, downloadsDirPath)
	if err != nil {
		s.Fatal("Failed to get the project ID: ", err)
	}
	if projectID != projectIDExtMediaImage {
		s.Errorf("Unexpected project ID: %d, expected %d",
			projectID, projectIDExtMediaImage)
	}
}
