// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
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
		Timeout:      10 * time.Minute,
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
	// These numbers come from Android's android_projectid_config.h.
	const (
		projectIDExtMediaImage = 1003
		projectIDExtDataStart  = 20000
		projectIDExtDataEnd    = 29999
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

	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

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

	// Check the project ID of the file in the external files dir.
	pkgDataDir, err := arc.PkgDataDir(cr.NormalizedUser(), pkgName)
	if err != nil {
		s.Fatal("Failed to get package data dir: ", err)
	}
	externalFilesDirPath := filepath.Join(pkgDataDir, "files/Pictures/test.png")
	projectID, err := getQuotaProjectID(ctx, externalFilesDirPath)
	if err != nil {
		s.Fatal("Failed to get the project ID: ", err)
	}
	if projectID < projectIDExtDataStart || projectIDExtDataEnd < projectID {
		s.Error("Unexpected project ID ", projectID)
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
		s.Error("Unexpected project ID ", projectID)
	}
}
