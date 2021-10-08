// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package uiauto

import (
	"fmt"
	"image/draw"
	"io"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/matts1/vnc2video"
	"github.com/matts1/vnc2video/encoders"

	"chromiumos/tast/errors"
)

// createVideo uses VNC to generate images, and then pipes the images into ffmpeg to generate video.
func createVideo(s testingState, enc *vnc2video.TightEncoding, canvas draw.Image, startTime, endTime time.Time, cfg videoConfig) error {
	cmd := exec.Command("ffmpeg",
		// These arguments are required to encode the video correctly.
		"-f", "image2pipe",
		"-vcodec", "ppm",
		"-r", fmt.Sprint(cfg.framerate),
		"-an",     //no audio
		"-i", "-", // Take input frames from stdin.
		"-vcodec", "libvpx",

		// Note: significant potential improvements available in encoding speed by
		// switching to vsync 1, but it requires attaching a timestamp to the
		// stream, so will need to work out the encoding format.
		// vsync 2 means that frames have no timestamp.
		"-vsync", "2",

		// The following are tuning parameters for performance / quality optimisation.
		"-probesize", "10000000", // Appears to have no noticeable impact on performance.
		"-b:v", "0.5M",
		"-threads", "0", // Let ffmpeg decide how many threads to use.
		"-tile-columns", "6",
		"-frame-parallel", "1",
		"-quality", "good",
		// https://www.webmproject.org/docs/encoder-parameters/
		// Encoding the same 5 second video on my DUT results in the following results.
		// 0 = 4.7s, 122KB
		// 1 = 3.1s, 316KB
		// 2 = 2.7s  306KB
		// 3 = 1.7s  276KB
		// 4 = 1.4s  304KB
		// 5 = 0.8s  314KB
		// For this use case, the video quality is perfectly acceptable.
		"-cpu-used", "5",
		"-g", "180",
		"-keyint_min", "180",
		"-qmax", "51",
		"-qmin", "3",
		filepath.Join(s.OutDir(), cfg.fileName))

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	cleanup := func(initialErr error) error {
		stdin.Close()
		// 64KB should be plenty to store the error message.
		buf := make([]byte, 65536)
		// Ignore error from io.ReadFull. If we can't read an error message,
		// there's not much we can do.
		io.ReadFull(stderr, buf)
		if err := cmd.Wait(); err != nil {
			return errors.Errorf("ffmpeg failed to encode video: %s", string(buf))
		}
		return initialErr
	}

	for t := startTime; t.Before(endTime); t = t.Add(time.Second / time.Duration(cfg.framerate)) {
		if err := enc.DrawUntilTime(canvas, t); err != nil {
			return cleanup(err)
		}
		if err := encoders.EncodePPM(stdin, canvas); err != nil {
			return cleanup(err)
		}
	}

	return cleanup(nil)
}
