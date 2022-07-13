// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"path/filepath"
	"strings"
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
	cameraAppActivity    = ".CameraActivity"
	cameraAppApk         = "ArcCameraFpsTest.apk"
	cameraAppPackage     = "org.chromium.arc.testapp.camerafps"
	intentGetCameraIDs   = "org.chromium.arc.testapp.camerafps.ACTION_GET_CAMERA_IDS"
	intentSetCameraID    = "org.chromium.arc.testapp.camerafps.ACTION_SET_CAMERA_ID"
	intentTakePhoto      = "org.chromium.arc.testapp.camerafps.ACTION_TAKE_PHOTO"
	intentStartRecording = "org.chromium.arc.testapp.camerafps.ACTION_START_RECORDING"
	intentStopRecording  = "org.chromium.arc.testapp.camerafps.ACTION_STOP_RECORDING"
	intentResetCamera    = "org.chromium.arc.testapp.camerafps.ACTION_RESET_CAMERA"

	// Snapshots can be really small if the room is dark, but JPEGs are never smaller than 100 bytes.
	minExpectedFileSize = 100
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ArcCameraApp,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks basic Android camera functionalities works under ARC",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Fixture:      "arcBootedRestricted",
	})
}

func parseCameraIds(ctx context.Context, raw string) []string {
	// Format: [0: {CamId0}, 1: {CamId1}]. Example: [0: 0, 1: 1].
	pairs := strings.Split(raw[1:len(raw)-1], ", ")
	var ids []string
	for _, pair := range pairs {
		values := strings.Split(pair, ": ")
		if len(values) > 1 {
			ids = append(ids, values[1])
		}
	}
	return ids
}

func ArcCameraApp(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	a := s.FixtValue().(*arc.PreData).ARC
	if err := a.Install(ctx, arc.APKPath(cameraAppApk)); err != nil {
		s.Fatal("Failed to install the APK: ", err)
	}

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

			rawCameraIds, err := a.BroadcastIntentGetData(ctx, intentGetCameraIDs)
			if err != nil {
				s.Fatal("Failed to get camera ids: ", err)
			}
			cameraIds := parseCameraIds(ctx, rawCameraIds)

			for _, id := range cameraIds {
				if _, err := a.BroadcastIntent(ctx, intentSetCameraID, "--ei", "id", id); err != nil {
					s.Fatalf("Failed to switch to camera: %v, %v", id, err)
				}

				if _, err := a.BroadcastIntent(ctx, intentResetCamera); err != nil {
					s.Fatal("Failed to reset camera: ", err)
				}

				if err := tst.testFunc(ctx, cr, a); err != nil {
					s.Fatalf("Failed when running sub test %v: %v", tst.name, err)
				}
			}
		})
		cancel()
	}
}

func takePhoto(ctx context.Context, cr *chrome.Chrome, a *arc.ARC) error {
	outputFile, err := a.BroadcastIntentGetData(ctx, intentTakePhoto)
	if err != nil {
		return errors.Wrap(err, "could not send intent")
	}

	filePath := filepath.Join("files/DCIM", outputFile)
	testing.ContextLog(ctx, "Output file: ", filePath)
	// Check if photo file was generated.
	if fileSize, err := arc.PkgFileSize(ctx, cr.NormalizedUser(), cameraAppPackage, filePath); err != nil {
		return errors.Wrap(err, "could not determine size of photo file")
	} else if fileSize < minExpectedFileSize {
		return errors.Wrapf(err, "photo file is smaller than expected: got %d, want >= %d", fileSize, minExpectedFileSize)
	}
	return nil
}

func recordVideo(ctx context.Context, cr *chrome.Chrome, a *arc.ARC) error {
	// Start record video
	outputFile, err := a.BroadcastIntentGetData(ctx, intentStartRecording)
	if err != nil {
		return errors.Wrap(err, "could not send intent")
	}
	filePath := filepath.Join("files/DCIM", outputFile)
	testing.ContextLog(ctx, "Output file: ", filePath)

	testing.Sleep(ctx, 5*time.Second)
	if _, err = a.BroadcastIntent(ctx, intentStopRecording); err != nil {
		return errors.Wrap(err, "could not send intent")
	}
	// Check if video file was generated.
	if fileSize, err := arc.PkgFileSize(ctx, cr.NormalizedUser(), cameraAppPackage, filePath); err != nil {
		return errors.Wrap(err, "could not determine size of video file")
	} else if fileSize < minExpectedFileSize {
		return errors.Wrapf(err, "video file is smaller than expected: got %d, want >= %d", fileSize, minExpectedFileSize)
	}
	return nil
}
