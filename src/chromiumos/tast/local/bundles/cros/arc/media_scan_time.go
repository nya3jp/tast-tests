// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
	"context"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"time"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MediaScanTime,
		Desc:         "Checks elapsed time during full-volume media scan",
		Contacts:     []string{"risan@chromium.org", "youkichihosoi@chromium.org", "arc-storage@google.com"},
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
	})
}
func RemountDirectory(ctx context.Context, s *testing.State, a *arc.ARC, cr *chrome.Chrome, directoryName string, filename string, makecopy func(*testing.State, *chrome.Chrome, string)) {
	// Unmount sdcard directory.
	if err := a.Command(ctx, "sm", "unmount", directoryName).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to unmount "+directoryName+": ", err)
	}
	makecopy(s, cr, filename)
	// Remount sdcard directory.
	if err := a.Command(ctx, "sm", "mount", directoryName).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to mount "+directoryName+": ", err)
	}
}
func copyFileToTargetPath(s *testing.State, targetDir string, filename string) {
	capybaraPath, err := ioutil.ReadFile(s.DataPath(filename))
	if err != nil {
		s.Fatal("Failed to read the test file: ", err)
	}
	targetPath := filepath.Join(targetDir, filename)
	if err := ioutil.WriteFile(targetPath, capybaraPath, 0666); err != nil {
		s.Fatal("Failed to copy the test file: ", err)
	}
}

func populateFilesInSdcard(s *testing.State, cr *chrome.Chrome, filename string) {
	cryptohomeSystemPath, err := cryptohome.SystemPath(cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get the cryptohome directory: ", err)
	}
	targetDir := filepath.Join(cryptohomeSystemPath, "android-data", "data", "media", "0", "Pictures")
	copyFileToTargetPath(s, targetDir, filename)
}

func populateFilesInMyFiles(ctx context.Context, s *testing.State, cr *chrome.Chrome, filename string) {
	cryptohomeUserPath, err := cryptohome.UserPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get the cryptohome directory: ", err)
	}
	targetDir := filepath.Join(cryptohomeUserPath, "MyFiles")
	copyFileToTargetPath(s, targetDir, filename)
}

func getElapsedTimeData(ctx context.Context, d *ui.Device, pkg string) (float64, error) {
	view := d.Object(ui.ID(pkg + ":id/file_content"))
	var elapsedTime float64
	if err := view.WaitForExists(ctx, 10*time.Second); err != nil {
		return elapsedTime, err
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		text, err := view.GetText(ctx)
		if err != nil {
			return err
		}
		elapsedTime, err = strconv.ParseFloat(text, 64)
		if err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second}); err != nil {
		return elapsedTime, err
	}
	return elapsedTime, nil
}

func MediaScanTime(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome
	d := s.FixtValue().(*arc.PreData).UIDevice
	tconn, err := cr.TestAPIConn(ctx)
	const (
		apk      = "ArcMediaScanTimeTest.apk"
		pkg      = "org.chromium.arc.testapp.mediascantime"
		cls      = "org.chromium.arc.testapp.mediascantime.MainActivity"
		filename = "capybara.jpg"
	)

	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to create a new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start the activity: ", err)
	}

	RemountDirectory(ctx, s, a, cr, "emulated;0", filename, populateFilesInSdcard)
	time, err := getElapsedTimeData(ctx, d, pkg)
	if err != nil {
		s.Fatal("Failed to get data from app UI: ", err)
	}

	perfValues := perf.NewValues()
	perfValues.Set(perf.Metric{
		Name:      "mediaScanTime",
		Unit:      "msec",
		Direction: perf.SmallerIsBetter,
	}, time)

	if err := perfValues.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}
