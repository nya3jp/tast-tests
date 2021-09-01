// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

const (
	mediaScanTimeMyFilesUUID = "0000000000000000000000000000CAFEF00D2019"
	mediaScanTimePkg         = "org.chromium.arc.testapp.mediascantime"
	capybaraFileName         = "capybara.jpg"
)

type arcMediaScanTimeParams struct {
	getTargetDir func(ctx context.Context, cr *chrome.Chrome) (string, error)
	getVolumeID  func(ctx context.Context, a *arc.ARC) (string, error)
	uriSuffix    string
}

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
			Name:              "sdcard",
			ExtraSoftwareDeps: []string{"android_p"},
			Val: arcMediaScanTimeParams{
				getTargetDir: getSdcardTargetDir,
				getVolumeID:  getSdcardVolumeID,
				uriSuffix:    "emulated/0",
			},
		}, {
			Name:              "sdcard_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: arcMediaScanTimeParams{
				getTargetDir: getSdcardTargetDir,
				getVolumeID:  getSdcardVolumeID,
				uriSuffix:    "emulated/0",
			},
		}, {
			Name:              "myfiles",
			ExtraSoftwareDeps: []string{"android_p"},
			Val: arcMediaScanTimeParams{
				getTargetDir: getMyFilesTargetDir,
				getVolumeID:  getMyFilesVolumeID,
				uriSuffix:    mediaScanTimeMyFilesUUID,
			},
		}, {
			Name:              "myfiles_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: arcMediaScanTimeParams{
				getTargetDir: getMyFilesTargetDir,
				getVolumeID:  getMyFilesVolumeID,
				uriSuffix:    mediaScanTimeMyFilesUUID,
			},
		}},
	})
}

func getSdcardTargetDir(ctx context.Context, cr *chrome.Chrome) (string, error) {
	cryptohomeSystemPath, err := cryptohome.SystemPath(cr.NormalizedUser())
	if err != nil {
		return "", errors.New("sdcard directory path not found")
	}
	return filepath.Join(cryptohomeSystemPath, "android-data", "data", "media", "0", "Pictures"), nil
}

func getMyFilesTargetDir(ctx context.Context, cr *chrome.Chrome) (string, error) {
	cryptohomeUserPath, err := cryptohome.UserPath(ctx, cr.NormalizedUser())
	if err != nil {
		return "", errors.New("MyFiles directory path not found")
	}
	return filepath.Join(cryptohomeUserPath, "MyFiles"), nil
}

func getSdcardVolumeID(ctx context.Context, a *arc.ARC) (string, error) {
	return "emulated;0", nil
}

func getMyFilesVolumeID(ctx context.Context, a *arc.ARC) (string, error) {
	re := regexp.MustCompile(`^(stub:)?[0-9]+\s+mounted\s+` + mediaScanTimeMyFilesUUID + `$`)
	out, err := a.Command(ctx, "sm", "list-volumes").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", err
	}
	lines := bytes.Split(out, []byte("\n"))
	for _, line := range lines {
		if volumeIDLine := re.Find(bytes.TrimSpace(line)); volumeIDLine != nil {
			splitVolumeIDLine := strings.Split(string(volumeIDLine), " ")
			return splitVolumeIDLine[0], nil
		}
	}
	return "", errors.New("MyFiles volume not found")
}

func remountDirectory(ctx context.Context, s *testing.State, a *arc.ARC, cr *chrome.Chrome, volumeID, targetDir string) {
	if err := a.Command(ctx, "sm", "unmount", volumeID).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to unmount "+volumeID+": ", err)
	}
	copyFileToTargetPath(s, targetDir)
	if err := a.Command(ctx, "sm", "mount", volumeID).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to mount "+volumeID+": ", err)
	}
}

func copyFileToTargetPath(s *testing.State, targetDir string) {
	capybara, err := ioutil.ReadFile(s.DataPath(capybaraFileName))
	if err != nil {
		s.Fatal("Failed to read the test file: ", err)
	}
	for i := 0; i < 100; i++ {
		targetPath := filepath.Join(targetDir, "capybara_"+strconv.Itoa(i)+".jpg")
		if err := ioutil.WriteFile(targetPath, capybara, 0666); err != nil {
			s.Fatal("Failed to copy the test file: ", err)
		}
	}
}

func getElapsedTimeData(ctx context.Context, d *ui.Device) (float64, error) {
	view := d.Object(ui.ID(mediaScanTimePkg + ":id/media_scan_time"))
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
		apk       = "ArcMediaScanTimeTest.apk"
		cls       = "org.chromium.arc.testapp.mediascantime.MainActivity"
		uriPrefix = "file:///storage/"
	)

	mediaScanTimeOpt := s.Param().(arcMediaScanTimeParams)

	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	act, err := arc.NewActivity(a, mediaScanTimePkg, cls)
	if err != nil {
		s.Fatal("Failed to create a new activity: ", err)
	}
	defer act.Close()
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start the activity: ", err)
	}
	expectedURI := uriPrefix + mediaScanTimeOpt.uriSuffix
	if err := act.StartWithArgs(ctx, tconn, []string{"-S", "-W", "-n"}, []string{"-d", expectedURI}); err != nil {
		s.Fatal("Failed to start the activity: ", err)
	}
	targetDir, err := mediaScanTimeOpt.getTargetDir(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get the path to target: ", err)
	}
	volumeID, err := mediaScanTimeOpt.getVolumeID(ctx, a)
	if err != nil {
		s.Fatal("Failed to get volume ID: ", err)
	}
	remountDirectory(ctx, s, a, cr, volumeID, targetDir)
	time, err := getElapsedTimeData(ctx, d)
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
