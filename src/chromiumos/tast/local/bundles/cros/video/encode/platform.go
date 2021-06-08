// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package encode

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/coords"
)

var ym12Detect = regexp.MustCompile(`'YM12'`)
var nv12Detect = regexp.MustCompile(`'NV12'`)

// CommandBuilderFn is the function type to generate the command line with arguments.
type CommandBuilderFn func(ctx context.Context, exe, yuvFile string, size coords.Size, fps int) (command []string, ivfFile string, bitrate int, err error)

// RunCommand runs the exe binary test with arguments args.
func RunCommand(ctx context.Context, outDir, exe string, args ...string) (logFile string, err error) {
	logFile = filepath.Join(outDir, filepath.Base(exe)+".txt")
	f, err := os.Create(logFile)
	if err != nil {
		return "", errors.Wrap(err, "failed to create log file")
	}
	defer f.Close()

	cmd := testexec.CommandContext(ctx, exe, args...)
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return "", errors.Wrapf(err, "failed to run: %s", exe)
	}
	return logFile, nil
}

// VP8EncArgsVAAPI constructs the command line for the VP8 encoding binary exe.
func VP8EncArgsVAAPI(ctx context.Context, exe, yuvFile string, size coords.Size, fps int) (command []string, ivfFile string, bitrate int, _ error) {
	command = append(command, exe, strconv.Itoa(size.Width), strconv.Itoa(size.Height), yuvFile)

	ivfFile = yuvFile + ".ivf"
	command = append(command, ivfFile)

	// WebRTC uses Constant BitRate (CBR) with a very large intra-frame
	// period, error resiliency and a certain quality parameter and target
	// bitrate.
	command = append(command, "--intra_period", "3000")
	command = append(command, "--qp", "28" /* Quality Parameter */)
	command = append(command, "--rcmode", "1" /* For Constant BitRate (CBR) */)
	command = append(command, "--error_resilient" /* Off by default, enable. */)

	command = append(command, "-f", strconv.Itoa(fps))

	bitrate = int(0.1 /* BPP */ * float64(fps) * float64(size.Width) * float64(size.Height))
	command = append(command, "--fb", strconv.Itoa(bitrate) /* Kbps */)
	return
}

// VP9EncArgsVAAPI constructs the command line for the VP9 encoding binary exe.
func VP9EncArgsVAAPI(ctx context.Context, exe, yuvFile string, size coords.Size, fps int) (command []string, ivfFile string, bitrate int, _ error) {
	command = append(command, exe, strconv.Itoa(size.Width), strconv.Itoa(size.Height), yuvFile)

	ivfFile = yuvFile + ".ivf"
	command = append(command, ivfFile)

	// WebRTC uses Constant BitRate (CBR) with a very large intra-frame
	// period and a certain quality parameter target, loop filter level
	// and bitrate.
	command = append(command, "--intra_period", "3000")
	command = append(command, "--qp", "24" /* Quality Parameter */)
	command = append(command, "--rcmode", "1" /* For Constant BitRate (CBR) */)
	command = append(command, "--lf_level", "10" /* Loop filter level. */)

	// Intel Gen 11 and later (JSL, TGL, etc) only support Low-Power
	// encoding. Let exe decide which one to use (auto mode).
	command = append(command, "--low_power", "-1")

	command = append(command, "-f", strconv.Itoa(fps))

	// VP9 uses a 30% better bitrate than VP8/H.264, which targets 0.1 bpp.
	bitrate = int(0.07 /* BPP */ * float64(fps) * float64(size.Width) * float64(size.Height))
	command = append(command, "--fb", strconv.Itoa(bitrate/1000.0) /* in Kbps */)
	return
}

// H264EncodeArgsVAAPI constructs the command line for the H.264 encoding binary exe.
func H264EncodeArgsVAAPI(ctx context.Context, exe, yuvFile string, size coords.Size, fps int) (command []string, h264File string, bitrate int, _ error) {
	command = append(command, exe, "-w", strconv.Itoa(size.Width), "-h", strconv.Itoa(size.Height))
	command = append(command, "--srcyuv", yuvFile, "--fourcc", "YV12")
	command = append(command, "-n", "0" /* Read number of frames from yuvFile*/)

	h264File = yuvFile + ".h264"
	command = append(command, "-o", h264File)

	// WebRTC uses Constant BitRate (CBR) with a very large intra-frame
	// period and a certain quality parameter target, bitrate and profile.
	command = append(command, "--intra_period", "2048", "--idr_period", "2048", "--ip_period", "1")
	command = append(command, "--minqp", "24", "--initialqp", "26" /* Quality Parameter */)
	command = append(command, "--profile", "BP" /* (Constrained) Base Profile. */)

	command = append(command, "-f", strconv.Itoa(fps))

	command = append(command, "--rcmode", "CBR" /* Constant BitRate */)
	bitrate = int(0.1 /* BPP */ * float64(fps) * float64(size.Width) * float64(size.Height))
	command = append(command, "--bitrate", strconv.Itoa(bitrate) /* bps */)
	return
}

// V4L2StatefulEncoderArgsH264 constructs the command line for the v4l2_stateful_encoder and for H.264.
func V4L2StatefulEncoderArgsH264(ctx context.Context, exe, yuvFile string, size coords.Size, fps int) (command []string, h264File string, bitrate int, err error) {
	command = append(command, exe, "--width", strconv.Itoa(size.Width), "--height", strconv.Itoa(size.Height))
	command = append(command, "--file", yuvFile, "--file_format", "yv12")

	command = append(command, "--fps", strconv.Itoa(fps))

	command = append(command, "--codec", "H264")

	// The output file automatically gets a .h264 suffix added.
	command = append(command, "--output", yuvFile)
	h264File = yuvFile + ".h264"

	// WebRTC uses Constant BitRate (CBR) with a very large intra-frame
	// period and a certain quality parameter target, bitrate and profile.
	command = append(command, "--gop", "65535")
	command = append(command, "--end_usage", "CBR" /* Constant BitRate */)

	bitrate = int(0.1 /* BPP */ * float64(fps) * float64(size.Width) * float64(size.Height))
	command = append(command, "--bitrate", strconv.Itoa(bitrate) /* bps */)

	// Query the driver for its supported input (OUTPUT_queue) video pixel formats.
	v4l2CtlCmd := testexec.CommandContext(ctx, "v4l2-ctl", "--device",
		"/dev/video-enc0", "--list-formats-out")
	v4l2Out, err := v4l2CtlCmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, "", 0, errors.Wrap(err, "failed to run v4l2-ctl to query OUTPUT formats")
	}
	v4l2Lines := strings.Split(string(v4l2Out), "\n")
	// If the pixel format is not listed below, we leave it unspecified for exe to
	// figure out. For more information on these pixel formats see:
	// https://www.kernel.org/doc/html/v5.4/media/uapi/v4l/yuv-formats.html.
	for _, line := range v4l2Lines {
		if ym12Detect.MatchString(line) {
			command = append(command, "--buffer_fmt", "YM12")
		} else if nv12Detect.MatchString(line) {
			command = append(command, "--buffer_fmt", "NV12")
		}
	}
	return
}
