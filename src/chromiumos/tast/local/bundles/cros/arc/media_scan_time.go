// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"io"
	"os"
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
	// To clarify the difference in elapsed time due to performance,
	// we need to populate files under the target directory. On the other hand,
	// taking a long to create many files obstruct is troublesome at least
	// under checking the test's behavior. Therefore, 100 is suitable for the number of
	// files copied under the directory.
	numberOfCopies = 100
)

type arcMediaScanTimeParams struct {
	getTargetDir    func(ctx context.Context, user string) (string, error)
	getVolumeID     func(ctx context.Context, a *arc.ARC) (string, error)
	volumeURISuffix string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         MediaScanTime,
		Desc:         "Checks elapsed time during full-volume media scan",
		Contacts:     []string{"risan@chromium.org", "youkichihosoi@chromium.org", "arc-storage@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Data:         []string{capybaraFileName},
		Params: []testing.Param{{
			Name:              "sdcard",
			ExtraSoftwareDeps: []string{"android_p"},
			Val: arcMediaScanTimeParams{
				getTargetDir:    getSdcardTargetDir,
				getVolumeID:     getSdcardVolumeID,
				volumeURISuffix: "emulated/0",
			},
		}, {
			Name:              "sdcard_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: arcMediaScanTimeParams{
				getTargetDir:    getSdcardTargetDir,
				getVolumeID:     getSdcardVolumeID,
				volumeURISuffix: "emulated/0",
			},
		}, {
			Name:              "myfiles",
			ExtraSoftwareDeps: []string{"android_p"},
			Val: arcMediaScanTimeParams{
				getTargetDir:    getMyFilesTargetDir,
				getVolumeID:     getMyFilesVolumeID,
				volumeURISuffix: mediaScanTimeMyFilesUUID,
			},
		}, {
			Name:              "myfiles_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: arcMediaScanTimeParams{
				getTargetDir:    getMyFilesTargetDir,
				getVolumeID:     getMyFilesVolumeID,
				volumeURISuffix: mediaScanTimeMyFilesUUID,
			},
		}},
	})
}

func getSdcardTargetDir(ctx context.Context, user string) (string, error) {
	androidDataDir, err := arc.AndroidDataDir(user)
	if err != nil {
		return "", errors.Wrap(err, "failed to get android-data path for user "+user)
	}
	return filepath.Join(androidDataDir, "data", "media", "0", "Pictures", "capybaras"), nil
}

func getMyFilesTargetDir(ctx context.Context, user string) (string, error) {
	cryptohomeUserPath, err := cryptohome.UserPath(ctx, user)
	if err != nil {
		return "", errors.Wrap(err, "failed to get cryptohome user path for user "+user)
	}
	return filepath.Join(cryptohomeUserPath, "MyFiles", "capybaras"), nil
}

func getSdcardVolumeID(ctx context.Context, a *arc.ARC) (string, error) {
	return "emulated;0", nil
}

func getMyFilesVolumeID(ctx context.Context, a *arc.ARC) (string, error) {
	// Regular expression that matches the output line for the mounted
	// MyFiles volume. Each output line of the sm command is of the form:
	// <volume id><space(s)><mount status><space(s)><volume UUID>.
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

func populateFilesAndRemountDirectory(ctx context.Context, s *testing.State, a *arc.ARC, cr *chrome.Chrome, volumeID, targetDir string) {
	populateFilesUnderTargetPath(s, targetDir)
	re := regexp.MustCompile(`Row:\s+[0-9]+\s+_data=/storage/0000000000000000000000000000CAFEF00D2019/capybaras/capybara_[0-9]+.jpg$`)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := a.Command(ctx, "content", "query", "--uri", "content://media/external/file", "--projection", "_data").Output(testexec.DumpLogOnError)
		if err != nil {
			return err
		}
		lines := bytes.Split(out, []byte("\n"))
		var numberOfScannedFiles int
		for _, line := range lines {
			if mediaStoreQueryLine := re.Find(bytes.TrimSpace(line)); mediaStoreQueryLine != nil {
				numberOfScannedFiles++
			}
		}
		if numberOfScannedFiles < numberOfCopies {
			return errors.New("Not enough capybara copies")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to scan and index all copied files by Android's MediaProvider: ", err)
	}
	if err := a.Command(ctx, "content", "delete", "--uri", "content://media/external/file?deletedata=false").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to delete all copied files from Android's MediaStore database: ", err)
	}
	if err := a.Command(ctx, "sm", "unmount", volumeID).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to unmount "+volumeID+": ", err)
	}
	if err := a.Command(ctx, "sm", "mount", volumeID).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to mount "+volumeID+": ", err)
	}
}

func populateFilesUnderTargetPath(s *testing.State, targetDir string) {
	capybaraSrc, err := os.Open(s.DataPath(capybaraFileName))
	if err != nil {
		s.Fatal("Failed to read the test file: ", err)
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		s.Fatal("Failed to create the target dir "+targetDir+": ", err)
	}
	for i := 0; i < numberOfCopies; i++ {
		dstPath := filepath.Join(targetDir, "capybara_"+strconv.Itoa(i)+".jpg")
		dst, err := os.Create(dstPath)
		if err != nil {
			s.Fatal("Failed to create "+dstPath+", file for copying capybara image : ", err)
		}
		defer dst.Close()

		if _, err := io.Copy(dst, capybaraSrc); err != nil {
			s.Fatal("Failed to copy the test file to "+dstPath+": ", err)
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

func startMeasureMediaScanTimeWithApp(ctx context.Context, s *testing.State, a *arc.ARC, tconn *chrome.TestConn, volumeURI string) (func(), error) {
	const (
		apk = "ArcMediaScanTimeTest.apk"
		cls = "org.chromium.arc.testapp.mediascantime.MainActivity"
	)

	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		return nil, err
	}

	act, err := arc.NewActivity(a, mediaScanTimePkg, cls)
	if err != nil {
		return nil, err
	}

	if err := act.StartWithArgs(ctx, tconn, []string{"-S", "-W", "-n"}, []string{"-d", volumeURI}); err != nil {
		act.Close()
		return nil, err
	}
	return act.Close, nil
}

func MediaScanTime(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome
	d := s.FixtValue().(*arc.PreData).UIDevice
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	param := s.Param().(arcMediaScanTimeParams)
	volumeURI := "file:///storage/" + param.volumeURISuffix

	closeApp, err := startMeasureMediaScanTimeWithApp(ctx, s, a, tconn, volumeURI)
	if err != nil {
		s.Fatal("Failed to start app: ", err)
	}
	defer closeApp()

	targetDir, err := param.getTargetDir(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get the path to target: ", err)
	}
	defer func() {
		if err := os.RemoveAll(targetDir); err != nil {
			s.Fatal("Failed to remove the directory created in the test: ", err)
		}
	}()

	volumeID, err := param.getVolumeID(ctx, a)
	if err != nil {
		s.Fatal("Failed to get volume ID: ", err)
	}

	populateFilesAndRemountDirectory(ctx, s, a, cr, volumeID, targetDir)

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
