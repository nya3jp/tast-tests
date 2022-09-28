// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	cameraAppActivity    = ".MainActivity"
	cameraAppApk         = "ArcCameraTest.apk"
	cameraAppPackage     = "chromeos.camera.app.arccameratest"
	intentSwitchCamera   = "chromeos.camera.app.arccameratest.ACTION_SWITCH_CAMERA"
	intentTakePhoto      = "chromeos.camera.app.arccameratest.ACTION_TAKE_PHOTO"
	intentStartRecording = "chromeos.camera.app.arccameratest.ACTION_START_RECORDING"
	intentStopRecording  = "chromeos.camera.app.arccameratest.ACTION_STOP_RECORDING"
	keyCameraFacing      = "chromeos.camera.app.arccameratest.KEY_CAMERA_FACING"

	// Snapshots can be really small if the room is dark, but JPEGs and MP4s are never smaller than 100 bytes.
	minExpectedFileSize = 100
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCCameraApp,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks basic Android camera functionalities work under ARC",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Fixture:      "arcBootedRestricted",
	})
}

func ARCCameraApp(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	a := s.FixtValue().(*arc.PreData).ARC
	if err := a.Install(ctx, arc.APKPath(cameraAppApk)); err != nil {
		s.Fatal("Failed to install the APK: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	// Prepare host-side access to Android's SDCard partition, which should store the generated photo and video files.
	cleanupFunc, err := arc.MountSDCardPartitionOnHostWithSSHFSIfVirtioBlkDataEnabled(ctx, a, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to make Android's SDCard partition available on host: ", err)
	}
	defer cleanupFunc(cleanupCtx)

	subTestTimeout := 30 * time.Second
	for _, tst := range []struct {
		name     string
		testFunc func(context.Context, *chrome.Chrome, *arc.ARC) error
	}{{
		"take_photo",
		takePhoto,
	}, {
		"record_video",
		recordVideo,
	}} {
		subTestCtx, cancel := context.WithTimeout(ctx, subTestTimeout)
		s.Run(subTestCtx, tst.name, func(ctx context.Context, s *testing.State) {
			cleanupCtx := ctx
			ctx, cancelCleanup := ctxutil.Shorten(ctx, 3*time.Second)
			defer cancelCleanup()

			activity, err := arc.NewActivity(a, cameraAppPackage, cameraAppActivity)
			if err != nil {
				s.Fatal("Failed to create new activity: ", err)
			}
			defer activity.Close()

			permissions := []string{
				"android.permission.CAMERA",
				"android.permission.RECORD_AUDIO",
				"android.permission.READ_EXTERNAL_STORAGE",
				"android.permission.WRITE_EXTERNAL_STORAGE"}
			for _, permission := range permissions {
				if err := a.Command(ctx, "pm", "grant", cameraAppPackage, permission).Run(testexec.DumpLogOnError); err != nil {
					s.Fatalf("Failed to grant permission %v: %v", permission, err)
				}
			}

			if err = activity.StartWithDefaultOptions(ctx, tconn); err != nil {
				s.Fatal("Failed to start app: ", err)
			}
			defer activity.Stop(cleanupCtx, tconn)

			for _, facing := range []string{"0", "1"} {
				testing.ContextLog(ctx, "Switch to camera ", facing)
				if success, err := a.BroadcastIntentGetData(ctx, intentSwitchCamera, "--ei", keyCameraFacing, facing); err != nil {
					s.Fatalf("Failed to switch to camera: %v, %v", facing, err)
				} else if success == "FALSE" {
					// Continue when there is no camera with such facing.
					continue
				}

				if err := tst.testFunc(ctx, cr, a); err != nil {
					s.Fatalf("Failed when running sub test %v: %v", tst.name, err)
				}
			}
		})
		cancel()
	}
}

// takePhoto asks ArcCameraFpsTest app to take a photo via intent and ensures
// that the captured photo is saved successfully.
func takePhoto(ctx context.Context, cr *chrome.Chrome, a *arc.ARC) error {
	outputFile, err := a.BroadcastIntentGetData(ctx, intentTakePhoto)
	if err != nil {
		return errors.Wrap(err, "could not send intent")
	}

	// Check if photo file was generated.
	if fileSize, err := fileSizeInDCIM(ctx, cr.NormalizedUser(), outputFile); err != nil {
		return errors.Wrap(err, "could not determine size of photo file")
	} else if fileSize < minExpectedFileSize {
		return errors.Wrapf(err, "photo file is smaller than expected: got %d, want >= %d", fileSize, minExpectedFileSize)
	}
	return nil
}

// recordVideo asks ArcCameraFpsTest app to record a video via intent and
// ensures that the captured video is saved successfully.
func recordVideo(ctx context.Context, cr *chrome.Chrome, a *arc.ARC) error {
	// Start record video
	if _, err := a.BroadcastIntent(ctx, intentStartRecording); err != nil {
		return errors.Wrap(err, "could not send intent")
	}

	testing.Sleep(ctx, 5*time.Second)
	outputFile, err := a.BroadcastIntentGetData(ctx, intentStopRecording)
	if err != nil {
		return errors.Wrap(err, "could not send intent")
	}

	// Check if video file was generated.
	if fileSize, err := fileSizeInDCIM(ctx, cr.NormalizedUser(), outputFile); err != nil {
		return errors.Wrap(err, "could not determine size of video file")
	} else if fileSize < minExpectedFileSize {
		return errors.Wrapf(err, "video file is smaller than expected: got %d, want >= %d", fileSize, minExpectedFileSize)
	}
	return nil
}

// fileSizeInDCIM searches the file inside Android DCIM folder and returns its size.
func fileSizeInDCIM(ctx context.Context, user, filename string) (int64, error) {
	androidDir, err := arc.AndroidDataDir(ctx, user)
	if err != nil {
		return -1, errors.Wrap(err, "failed to get Android data dir")
	}
	filePathInDCIM := filepath.Join(androidDir, "data/media/0/DCIM/", filename)

	info, err := os.Stat(filePathInDCIM)
	if err != nil {
		return -1, errors.Wrapf(err, "unable to access file: %s", filePathInDCIM)
	}
	return info.Size(), nil
}
