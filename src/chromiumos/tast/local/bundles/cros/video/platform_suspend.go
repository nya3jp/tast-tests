// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"context"
	"os"
	"strconv"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/video/encode"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/encoding"
	"chromiumos/tast/local/media/videotype"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// encodeCommandFn is the function type to generate the command line with arguments.
type encodeCommandFn func(ctx context.Context, exe, yuvFile string, size coords.Size, fps int) (command []string, ivfFile string, err error)

// suspendTest is used to describe the config used to run each test.
type suspendTest struct {
	command        string                  // The command path to be run. This should be relative to /usr/local/bin.
	filename       string                  // Input file name. This will be decoded to produce the uncompressed input to the encoder binary, so it can come in any format/container.
	fps            float64                 // FPS of the input file.
	size           coords.Size             // Width x Height in pixels of the input file.
	commandBuilder encode.CommandBuilderFn // Function to create the command line arguments.
}

func init() {
	testing.AddTest(&testing.Test{
		Func: PlatformSuspend,
		Desc: "Verifies suspend-command cycles do not hang/crash",
		Contacts: []string{
			"mcasas@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		Attr: []string{"group:graphics", "graphics_video", "graphics_perbuild"},
		Params: []testing.Param{{
			Name: "vaapi_vp8_360",
			Val: suspendTest{
				command:        "vp8enc",
				filename:       "tulip2-640x360.vp9.webm",
				fps:            30,
				size:           coords.NewSize(640, 360),
				commandBuilder: encode.VP8EncArgsVAAPI,
			},
			ExtraData:         []string{"tulip2-640x360.vp9.webm"},
			ExtraSoftwareDeps: []string{"vaapi", caps.HWEncodeVP8},
		}, {
			Name: "vaapi_vp9_360",
			Val: suspendTest{
				command:        "vp9enc",
				filename:       "tulip2-640x360.vp9.webm",
				fps:            30,
				size:           coords.NewSize(640, 360),
				commandBuilder: encode.VP9EncArgsVAAPI,
			},
			ExtraData:         []string{"tulip2-640x360.vp9.webm"},
			ExtraSoftwareDeps: []string{"vaapi", caps.HWEncodeVP9},
		}, {
			Name: "vaapi_h264_360",
			Val: suspendTest{
				command:        "h264encode",
				filename:       "tulip2-640x360.vp9.webm",
				fps:            30,
				size:           coords.NewSize(640, 360),
				commandBuilder: encode.H264EncodeArgsVAAPI,
			},
			ExtraData:         []string{"tulip2-640x360.vp9.webm"},
			ExtraSoftwareDeps: []string{"vaapi", caps.HWEncodeH264},
		}, {
			Name: "v4l2_h264_360",
			Val: suspendTest{
				command:        "v4l2_stateful_encoder",
				filename:       "tulip2-640x360.vp9.webm",
				fps:            30,
				size:           coords.NewSize(640, 360),
				commandBuilder: encode.V4L2StatefulEncoderArgsH264,
			},
			ExtraData:         []string{"tulip2-640x360.vp9.webm"},
			ExtraSoftwareDeps: []string{"v4l2_codec", caps.HWEncodeH264},
			// TODO(b/174103282): Enable on Rockchip devices.
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("bob", "gru", "kevin", "veyron_fievel", "veyron_tiger")),
		}},
		Timeout: 20 * time.Minute,
	})
}

// PlatformSuspend verifies that the GPU does not hang while going through
// several cycles of suspend-resume-command.
func PlatformSuspend(ctx context.Context, s *testing.State) {
	if err := upstart.EnsureJobRunning(ctx, "powerd"); err != nil {
		s.Fatal("Failed to ensure powerd job is started: ", err)
	}

	testOpt := s.Param().(suspendTest)

	yuvFile, err := encoding.PrepareYUV(ctx, s.DataPath(testOpt.filename), videotype.I420, coords.NewSize(0, 0) /* placeholder size */)
	if err != nil {
		s.Fatal("Failed to prepare YUV file: ", err)
	}
	defer os.Remove(yuvFile)

	command, encodedFile, _, err := testOpt.commandBuilder(ctx, testOpt.command, yuvFile, testOpt.size, int(testOpt.fps))
	if err != nil {
		s.Fatal("Failed to construct the command line: ", err)
	}

	const (
		delaySeconds   = 1
		timeoutSeconds = 2
		wakeupSeconds  = 2
	)

	var suspendCommand []string
	suspendCommand = append(suspendCommand, "powerd_dbus_suspend", "--delay="+strconv.Itoa(delaySeconds), "--timeout="+strconv.Itoa(timeoutSeconds), "--wakeup_timeout="+strconv.Itoa(wakeupSeconds))

	for i := 0; i < 10; i++ {
		s.Log("Running ", shutil.EscapeSlice(suspendCommand))
		_, err = encode.RunCommand(ctx, s.OutDir(), suspendCommand[0], suspendCommand[1:]...)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) { // Not an error
				break
			}

			if err := upstart.EnsureJobRunning(ctx, "powerd"); err != nil {
				s.Fatal("Failed to ensure powerd job is started: ", err)
			}
			//s.Fatal("Failed to run binary: ", err)
		}

		s.Log("Running ", shutil.EscapeSlice(command))
		_, err = encode.RunCommand(ctx, s.OutDir(), command[0], command[1:]...)
		if err != nil {
			s.Fatal("Failed to run binary: ", err)
		}
		defer os.Remove(encodedFile)
	}
}
