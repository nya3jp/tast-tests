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
	"strings"
	"strconv"
	"time"
	"chromiumos/tast/errors"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

const (
	mediaScanTimePkg      = "org.chromium.arc.testapp.mediascantime"
	capybaraFileName = "capybara.jpg"
)
type arcMediaScanTimeParams struct {
	isSdcard   bool
	UriSuffix  string
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
			Name:		   "sdcard",
			ExtraSoftwareDeps: []string{"android_p"},
			Val: arcMediaScanTimeParams{
				isSdcard:    true,
			},
		}, {
			Name:              "sdcard_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: arcMediaScanTimeParams{
				isSdcard:    true,
				UriSuffix:   "emulated/0",
			},
		}, {
			Name:		   "myfiles",
			ExtraSoftwareDeps: []string{"android_p"},
			Val: arcMediaScanTimeParams{
				isSdcard:    false,
			},
		}, {
			Name:		   "myfiles_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: arcMediaScanTimeParams{
				isSdcard:  false,
			},
		}},
	})
}

func getMyFilesVolumeId(ctx context.Context, a *arc.ARC) (string, error) {
	re := regexp.MustCompile(`^(stub:)?[0-9]+\s+mounted\s+` + myFilesUUID + `$`)
	out, err := a.Command(ctx, "sm", "list-volumes").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", err
	}
	lines := bytes.Split(out, []byte("\n"))
	for _, line := range lines {
		if volumeIdLine := re.Find(bytes.TrimSpace(line)); volumeIdLine != nil {
			splitVolumeIdLine := strings.Split(string(volumeIdLine), " ")
			return splitVolumeIdLine[0], nil
		}
	}
	return "", errors.New("MyFiles volume not found")
}

func RemountDirectory(ctx context.Context, s *testing.State, a *arc.ARC, cr *chrome.Chrome, directoryName string, targetDir string) {
	if err := a.Command(ctx, "sm", "unmount", directoryName).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to unmount "+directoryName+": ", err)
	}
	copyFileToTargetPath(s, targetDir)
	if err := a.Command(ctx, "sm", "mount", directoryName).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to mount "+directoryName+": ", err)
	}
}

func copyFileToTargetPath(s *testing.State, targetDir string) {
	capybara, err := ioutil.ReadFile(s.DataPath(capybaraFileName))
	if err != nil {
		s.Fatal("Failed to read the test file: ", err)
	}
	for i := 0; i < 100; i++ {
		targetPath := filepath.Join(targetDir, "capybara_" + strconv.Itoa(i) + ".jpg")
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
		apk      = "ArcMediaScanTimeTest.apk"
		cls      = "org.chromium.arc.testapp.mediascantime.MainActivity"
		UriPrefix = "file:///storage/"
	)

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
	expectedUri := UriPrefix + s.Param().(arcMediaScanTimeParams).UriSuffix
	if err := act.StartWithArgs(ctx, tconn, []string{"-S", "-W", "-n"}, []string{"-d", expectedUri}); err != nil {
		s.Fatal("Failed to start the activity: ", err)
	}
	if s.Param().(arcMediaScanTimeParams).isSdcard {
		cryptohomeSystemPath, err := cryptohome.SystemPath(cr.NormalizedUser())
		if err != nil {
			s.Fatal("Failed to get the cryptohome directory: ", err)
		}
		targetDir := filepath.Join(cryptohomeSystemPath, "android-data", "data", "media", "0", "Pictures")
		RemountDirectory(ctx, s, a, cr, "emulated;0", targetDir)
	} else {
		myFilesVolumeId, err := getMyFilesVolumeId(ctx, a)
		if err != nil {
			s.Fatal("Failed to get MyFiles volume id: ", err)
		}
		cryptohomeUserPath, err := cryptohome.UserPath(ctx, cr.NormalizedUser())
		if err != nil {
			s.Fatal("Failed to get the cryptohome directory: ", err)
		}
		targetDir := filepath.Join(cryptohomeUserPath, "MyFiles")
		RemountDirectory(ctx, s, a, cr, myFilesVolumeId, targetDir)
	}

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
