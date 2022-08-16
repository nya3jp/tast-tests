// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/media/devtools"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAFacingCameraVideoCapture,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies video capturing using user-facing/externally connected camera",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"video.html", "playback.js"},
		Fixture:      "ccaLaunched",
		Params: []testing.Param{{
			Name: "external_facing",
			Val:  cca.FacingExternal,
		}, {
			Name: "user_facing",
			Val:  cca.FacingFront,
		}},
	})
}

// CCAFacingCameraVideoCapture tests for capturing video from provided facing camera.
// Pre-requisite: For external facing camera connect USB.2 external camera to DUT before executing test.
func CCAFacingCameraVideoCapture(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(cca.FixtureData).Chrome
	app := s.FixtValue().(cca.FixtureData).App()
	defer cca.ClearSavedDir(cleanupCtx, cr)

	facingCamera := s.Param().(cca.Facing)

	// Check whether user facing camera switched.
	gotFacing, err := app.GetFacing(ctx)
	if err != nil {
		s.Fatal("Failed to get camera facing: ", err)
	}

	isFacingCamera := false
	maxCamera, err := app.GetNumOfCameras(ctx)
	if err != nil {
		s.Fatal("Failed to get available number of cameras: ", err)
	}

	for i := 0; i < maxCamera; i++ {
		if gotFacing != facingCamera {
			if err := app.SwitchCamera(ctx); err != nil {
				s.Fatal("Failed to switch camera: ", err)
			}
			gotFacing, err = app.GetFacing(ctx)
			if err != nil {
				s.Fatal("Failed to get facing after switching: ", err)
			}
		}
		if gotFacing == facingCamera {
			isFacingCamera = true
			break
		}
	}

	if !isFacingCamera {
		s.Fatalf("Failed to switch to camera: got %q; want %q", gotFacing, facingCamera)
	}

	if err := app.SwitchMode(ctx, cca.Video); err != nil {
		s.Fatal("Failed to switch to video mode: ", err)
	}

	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal("Failed to wait for video to active: ", err)
	}

	fileInfo, err := app.RecordVideo(ctx, cca.TimerOff, 3*time.Second)
	if err != nil {
		s.Fatal("Failed to record video: ", err)
	}

	videoPath, err := app.FilePathInSavedDir(ctx, fileInfo.Name())
	if err != nil {
		s.Fatal("Failed to get captured video path: ", err)
	}
	if videoPath == "" {
		s.Fatal("Failed: captured video path is empty")
	}

	dir, err := app.SavedDir(ctx)
	if err != nil {
		s.Fatal("Failed to get saved dir path: ", err)
	}

	if err := testexec.CommandContext(ctx, "cp", "-rf", s.DataPath("video.html"), dir).Run(); err != nil {
		s.Fatal("Failed to copy file: ", err)
	}

	if err := testexec.CommandContext(ctx, "cp", "-rf", s.DataPath("playback.js"), dir).Run(); err != nil {
		s.Fatal("Failed to copy file: ", err)
	}

	srv := httptest.NewServer(http.FileServer(http.Dir(dir)))
	defer srv.Close()

	url := srv.URL + "/video.html"
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		s.Fatal("Failed to load video.html: ", err)
	}
	defer conn.Close()

	videoFile := fileInfo.Name()
	if err := conn.Call(ctx, nil, "playUntilEnd", videoFile, true); err != nil {
		s.Fatal("Failed to play video: ", err)
	}

	observer, err := conn.GetMediaPropertiesChangedObserver(ctx)
	if err != nil {
		s.Fatal("Failed to retrieve DevTools Media messages: ", err)
	}

	isPlatform, decoderName, err := devtools.GetVideoDecoder(ctx, observer, url)
	if err != nil {
		s.Fatal("Failed to parse Media DevTools: ", err)
	}

	if !isPlatform {
		s.Fatal("Failed video decoder platform is not supported")
	}

	const (
		wantVaapiDecoder    = "VaapiVideoDecoder"
		wantPipelineDecoder = "VideoDecoderPipeline(ChromeOS)"
	)
	if decoderName != wantVaapiDecoder {
		if decoderName != wantPipelineDecoder {
			s.Fatalf("Failed: Hardware decoding accelerator was expected with decoder name but wasn't used: got: %q, want: %q or %q",
				decoderName, wantVaapiDecoder, wantPipelineDecoder)
		}
	}

	out, err := testexec.CommandContext(ctx, "vainfo").Output()
	if err != nil {
		s.Fatal("Failed to execute vainfo command: ", err)
	}
	videoCodecsRe := regexp.MustCompile(`VAProfileVP9Profile2.*VAEntrypointVLD`)
	if !videoCodecsRe.Match(out) {
		s.Fatal("Failed to find video codec from vainfo command output: ", err)
	}
}
