// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
	mediaScanPerfPkg = "org.chromium.arc.testapp.mediascanperf"
	capybaraFileName = "capybara.jpg"
	// numberOfCopies is the number of files to be copied to the target directory to be scanned.
	// The number should be large enough to get stable results. To clarify the difference
	// in elapsed time due to performance we need to create a lot of files under the target directory.
	numberOfCopies = 10000
)

type arcMediaScanPerfParams struct {
	targetDir            func(ctx context.Context, user string) (string, error)
	volumeID             func(ctx context.Context, a *arc.ARC) (string, error)
	waitForVolumeMount   func(ctx context.Context, a *arc.ARC) error
	waitForVolumeUnmount func(ctx context.Context, a *arc.ARC) error
	volumeURISuffix      string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         MediaScanPerf,
		Desc:         "Checks elapsed time during a full-volume media scan",
		Contacts:     []string{"risan@chromium.org", "youkichihosoi@chromium.org", "arc-storage@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_weekly"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Data:         []string{capybaraFileName},
		Timeout:      20 * time.Minute,
		Params: []testing.Param{{
			Name:              "sdcard_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: arcMediaScanPerfParams{
				targetDir:            sdCardTargetDir,
				volumeID:             arc.SDCardVolumeID,
				waitForVolumeMount:   arc.WaitForARCSDCardVolumeMount,
				waitForVolumeUnmount: arc.WaitForARCSDCardVolumeUnmount,
				volumeURISuffix:      "emulated/0",
			},
		}, {
			Name:              "myfiles_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: arcMediaScanPerfParams{
				targetDir:            myFilesTargetDir,
				volumeID:             arc.MyFilesVolumeID,
				waitForVolumeMount:   arc.WaitForARCMyFilesVolumeMount,
				waitForVolumeUnmount: arc.WaitForARCMyFilesVolumeUnmount,
				volumeURISuffix:      arc.MyFilesUUID,
			},
		}},
	})
}

// sdCardTargetDir gets the path to sdcard directory by arc.AndroidDataDir(), which will return a path in the format:
// /home/root/<hash>/android-data. Then, /data/media/0/Pictures/capybaras will be appended to the directory path
// to get the final path: /home/root/<hash>/android-data/data/media/0/Pictures.
func sdCardTargetDir(ctx context.Context, user string) (string, error) {
	androidDataDir, err := arc.AndroidDataDir(user)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get android-data path for user %s", user)
	}
	return filepath.Join(androidDataDir, "data", "media", "0", "Pictures", "capybaras"), nil
}

// myFilesTargetDir gets the path to MyFiles directory by cryptohome.UserPath(), which will return a path in the format:
// /home/user/<hash>. Then, /MyFiles/capybaras will be appended to the directory path
// to get the final path: /home/root/<hash>/MyFiles/capybaras.
func myFilesTargetDir(ctx context.Context, user string) (string, error) {
	cryptohomeUserPath, err := cryptohome.UserPath(ctx, user)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get cryptohome user path for user %s", user)
	}
	return filepath.Join(cryptohomeUserPath, "MyFiles", "capybaras"), nil
}

// createFileCopiesUnderTargetPath creates numberOfCopies copies of an image file under the target directory.
func createFileCopiesUnderTargetPath(s *testing.State, targetDir string) error {
	// Open the source file to be copied to the target directory.
	capybaraSrc, err := os.Open(s.DataPath(capybaraFileName))
	if err != nil {
		return errors.Wrapf(err, "failed to read the test file %s", capybaraFileName)
	}
	defer capybaraSrc.Close()

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return errors.Wrapf(err, "failed to create the target dir %s", targetDir)
	}

	// Create numberOfCopies copies of capybaraSrc in the target directory.
	for i := 0; i < numberOfCopies; i++ {
		dstPath := filepath.Join(targetDir, fmt.Sprintf("capybara_%d.jpg", i))
		dst, err := os.Create(dstPath)
		if err != nil {
			return errors.Wrapf(err, "failed to create %s, file for copying capybara image", dstPath)
		}
		if _, err := io.Copy(dst, capybaraSrc); err != nil {
			dst.Close()
			return errors.Wrapf(err, "failed to copy the test file to %s", dstPath)
		}
		dst.Close()
	}
	return nil
}

// waitForPopulatedFilesAddedToMediaStore waits for the newly created files under MyFiles to be scanned
// and added to MediaStore database.
func waitForPopulatedFilesAddedToMediaStore(ctx context.Context, a *arc.ARC) error {
	// Regular expression that matches the output line for the files added to MediaStore database.
	// Each output line of the sm command is of the form:
	// Row:<space(s)><row number><space(s)>_data=/storage/0000000000000000000000000000CAFEF00D2019/capybaras/capybara_<copy number>.jpg.
	re := regexp.MustCompile(`Row:\s+[0-9]+\s+_data=/storage/` + arc.MyFilesUUID + `/capybaras/capybara_[0-9]+.jpg`)
	prevNumberOfScannedFiles := -1

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Refer the list of files added to MediaStore database.
		out, err := a.Command(ctx, "content", "query", "--uri", "content://media/external/file", "--projection", "_data").Output(testexec.DumpLogOnError)
		if err != nil {
			return err
		}

		numberOfScannedFiles := len(re.FindAll(out, -1))
		testing.ContextLogf(ctx, "%d / %d files are scanned", numberOfScannedFiles, numberOfCopies)

		// All the files are scanned and added to MediaStore database.
		if numberOfScannedFiles >= numberOfCopies {
			return nil
		}

		// Sometimes MediaScan doesn't scan all the newly created files unexpectedly.
		if numberOfScannedFiles == prevNumberOfScannedFiles {
			return testing.PollBreak(errors.Errorf("media scan stops scanning image files: want %d; got %d", numberOfCopies, prevNumberOfScannedFiles))
		}
		prevNumberOfScannedFiles = numberOfScannedFiles

		return errors.Errorf("not enough image file copies: want %d; got %d", numberOfCopies, numberOfScannedFiles)
	}, &testing.PollOptions{Timeout: 20 * time.Minute, Interval: 3 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to scan and index all copied files by Android's MediaProvider")
	}
	return nil
}

// clearMediaStoreDB clears MediaStore database.
func clearMediaStoreDB(ctx context.Context, a *arc.ARC) error {
	testing.ContextLog(ctx, "Starting to clear MediaStore")
	return a.Command(ctx, "content", "delete", "--uri", "content://media/external/file?deletedata=false").Run(testexec.DumpLogOnError)
}

func unmountDirectory(ctx context.Context, a *arc.ARC, cr *chrome.Chrome, volumeID string) error {
	testing.ContextLog(ctx, "Starting to unmount ", volumeID)
	return a.Command(ctx, "sm", "unmount", volumeID).Run(testexec.DumpLogOnError)
}

func mountDirectory(ctx context.Context, a *arc.ARC, cr *chrome.Chrome, volumeID string) error {
	testing.ContextLog(ctx, "Starting to mount ", volumeID)
	return a.Command(ctx, "sm", "mount", volumeID).Run(testexec.DumpLogOnError)
}

// elapsedTimeData gets the elapsed time during full-volume media scan
// from Android app UI.
func elapsedTimeData(ctx context.Context, d *ui.Device) (float64, error) {
	view := d.Object(ui.ID(mediaScanPerfPkg + ":id/media_scan_perf"))
	testing.ContextLogf(ctx, "Waiting for a view %s matching the selector to appear", mediaScanPerfPkg)
	if err := view.WaitForExists(ctx, 5*time.Minute); err != nil {
		return 0.0, errors.Wrapf(err, "failed to wait for a view %s matching the selector to appear", mediaScanPerfPkg)
	}

	testing.ContextLog(ctx, "Waiting for getting the text content in app ui")
	var elapsedTime float64
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		text, err := view.GetText(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get the text content in app ui")
		}
		elapsedTime, err = strconv.ParseFloat(text, 64)
		if err != nil {
			return errors.Wrap(err, "failed to parse the text content in app ui to float64")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Minute}); err != nil {
		return elapsedTime, errors.Wrap(err, "failed to get data from app ui")
	}
	return elapsedTime, nil
}

// startMeasureMediaScanPerfWithApp installs ArcMediaScanPerfTest.apk, which measures
// the elapsed time during a full-volume media scan, and starts the app in DUT.
func startMeasureMediaScanPerfWithApp(ctx context.Context, a *arc.ARC, tconn *chrome.TestConn, volumeURI string) (func(), error) {
	const (
		apk = "ArcMediaScanPerfTest.apk"
		cls = "org.chromium.arc.testapp.mediascanperf.MainActivity"
	)

	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		return nil, err
	}

	act, err := arc.NewActivity(a, mediaScanPerfPkg, cls)
	if err != nil {
		return nil, err
	}

	if err := act.StartWithArgs(ctx, tconn, []string{"-S", "-W", "-n"}, []string{"-d", volumeURI}); err != nil {
		act.Close()
		return nil, err
	}
	return act.Close, nil
}

func MediaScanPerf(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome
	d := s.FixtValue().(*arc.PreData).UIDevice
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	param := s.Param().(arcMediaScanPerfParams)

	targetDirPath, err := param.targetDir(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get the path to target: ", err)
	}

	err = createFileCopiesUnderTargetPath(s, targetDirPath)
	// Cleanup should be called even if err is returned because some files might have been created.
	defer func() {
		if err := os.RemoveAll(targetDirPath); err != nil {
			s.Fatalf("Failed to remove the target directory %v: %v", targetDirPath, err)
		}
	}()
	if err != nil {
		s.Fatalf("Failed to create copies of %s under %v: %v", capybaraFileName, targetDirPath, err)
	}

	// ArcFileSystemWatcher is not attached to sdcard directory by bug (b/189416284),
	// therefore these steps can be skipped if the target directory is sdcard.
	if param.volumeURISuffix == arc.MyFilesUUID {
		if err := waitForPopulatedFilesAddedToMediaStore(ctx, a); err != nil {
			s.Fatal("Failed to wait for files to be added to MediaStore database: ", err)
		}
	}
	// If the newly created files are kept indexed, full-volume media scan excludes
	// them and the elapsed time will be shorter than expected.
	if err := clearMediaStoreDB(ctx, a); err != nil {
		s.Fatal("Failed to clear MediaStore database: ", err)
	}

	volumeID, err := param.volumeID(ctx, a)
	if err != nil {
		s.Fatal("Failed to get volume ID: ", err)
	}

	// Unmount the target directory volume. This step is needed for remounting to trigger a full-volume media scan later.
	if err := unmountDirectory(ctx, a, cr, volumeID); err != nil {
		s.Fatalf("Failed to unmount %s: %v", volumeID, err)
	}
	if err := param.waitForVolumeUnmount(ctx, a); err != nil {
		s.Fatalf("Failed to wait for %s to be unmounted: %v", volumeID, err)
	}

	volumeURI := "file:///storage/" + param.volumeURISuffix

	// Start Android app for receiving media scan intent and measuring the elapsed time.
	closeApp, err := startMeasureMediaScanPerfWithApp(ctx, a, tconn, volumeURI)
	if err != nil {
		s.Fatal("Failed to start app: ", err)
	}
	defer closeApp()

	// Mount the target directory volume. Media scan will be triggered and measurement will be started
	// just after mounting.
	if err := mountDirectory(ctx, a, cr, volumeID); err != nil {
		s.Fatalf("Failed to mount %s: %v", volumeID, err)
	}
	if err := param.waitForVolumeMount(ctx, a); err != nil {
		s.Fatalf("Failed to wait for %s to be mounted: %v", volumeID, err)
	}

	time, err := elapsedTimeData(ctx, d)
	if err != nil {
		s.Fatal("Failed to get data from app UI: ", err)
	}
	s.Logf("The elapsed time during a full volume media scan is %f msec", time)

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
