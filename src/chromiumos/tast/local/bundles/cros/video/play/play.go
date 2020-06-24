// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package play provides common code for playing videos on Chrome.
package play

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/bundles/cros/video/decode"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/logging"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// VideoType represents a type of video played in TestPlay.
type VideoType int

const (
	// NormalVideo represents a normal video. (i.e. non-MSE video.)
	NormalVideo VideoType = iota
	// MSEVideo represents a video requiring Media Source Extensions (MSE).
	MSEVideo
)

// VerifyHWAcceleratorMode represents a mode of TestPlay.
type VerifyHWAcceleratorMode int

const (
	// NoVerifyHWAcceleratorUsed is a mode that plays a video without verifying
	// hardware accelerator usage.
	NoVerifyHWAcceleratorUsed VerifyHWAcceleratorMode = iota
	// VerifyHWAcceleratorUsed is a mode that verifies a video is played using a
	// hardware accelerator.
	VerifyHWAcceleratorUsed
	// VerifyNoHWAcceleratorUsed is a mode that verifies a video is not played
	// using a hardware accelerator, i.e. it's using software decoding.
	VerifyNoHWAcceleratorUsed
)

// This is how long we need to wait before taking a screenshot in the
// TestPlayAndScreenshot case. This is necessary to ensure the video is on the screen
// and to let the "Press Esc to exit full screen" message disappear.
const delayToScreenshot = 7 * time.Second

// MSEDataFiles returns a list of required files that tests that play MSE videos.
func MSEDataFiles() []string {
	return []string{
		"shaka.html",
		"third_party/shaka-player/shaka-player.compiled.debug.js",
		"third_party/shaka-player/shaka-player.compiled.debug.map",
	}
}

// loadPage opens a new tab to load the specified webpage.
// Note that if err != nil, conn is nil.
func loadPage(ctx context.Context, cr *chrome.Chrome, url string) (*chrome.Conn, error) {
	ctx, st := timing.Start(ctx, "load_page")
	defer st.End()

	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open %v", url)
	}
	return conn, err
}

// playVideo invokes loadVideo(), plays a normal video in video.html, and checks if it has progress.
// videoFile is the file name which is played there.
// url is the URL of the video playback testing webpage.
func playVideo(ctx context.Context, cr *chrome.Chrome, videoFile, url string) error {
	ctx, st := timing.Start(ctx, "play_video")
	defer st.End()

	conn, err := loadPage(ctx, cr, url)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	if err := conn.EvalPromise(ctx, fmt.Sprintf("playUntilEnd(%q)", videoFile), nil); err != nil {
		return err
	}

	return nil
}

// playMSEVideo plays an MSE video stream via Shaka player, and checks its play progress.
// mpdFile is the name of MPD file for the video stream.
// url is the URL of the shaka player webpage.
func playMSEVideo(ctx context.Context, cr *chrome.Chrome, mpdFile, url string) error {
	ctx, st := timing.Start(ctx, "play_mse_video")
	defer st.End()

	conn, err := loadPage(ctx, cr, url)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	if err := conn.EvalPromise(ctx, fmt.Sprintf("play_shaka(%q)", mpdFile), nil); err != nil {
		return err
	}

	return nil
}

// seekVideoRepeatedly seeks video numSeeks times.
func seekVideoRepeatedly(ctx context.Context, conn *chrome.Conn, numSeeks int) error {
	ctx, st := timing.Start(ctx, "seek_video_repeatly")
	defer st.End()
	prevFinishedSeeks := 0
	for i := 0; i < numSeeks; i++ {
		finishedSeeks := 0
		if err := conn.EvalPromise(ctx, "randomSeek()", &finishedSeeks); err != nil {
			// If the test times out, EvalPromise() might be interrupted and return
			// zero finishedSeeks, in that case used the last known good amount.
			if finishedSeeks == 0 {
				finishedSeeks = prevFinishedSeeks
			}
			return errors.Wrapf(err, "Error while seeking, completed %d/%d seeks", finishedSeeks, numSeeks)
		}
		if finishedSeeks == numSeeks {
			break
		}
		prevFinishedSeeks = finishedSeeks
	}

	return nil
}

// playSeekVideo invokes loadVideo() then plays the video referenced by videoFile
// while repeatedly and randomly seeking into it numSeeks. It returns an error if
// seeking did not succeed for some reason.
// videoFile is the file name which is played and seeked there.
// baseURL is the base URL which serves video playback testing webpage.
func playSeekVideo(ctx context.Context, cr *chrome.Chrome, videoFile, baseURL string, numSeeks int) error {
	ctx, st := timing.Start(ctx, "play_seek_video")
	defer st.End()

	// Establish a connection to a video play page
	conn, err := loadPage(ctx, cr, baseURL+"/video.html")
	if err != nil {
		return err
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	if err := conn.EvalPromise(ctx, fmt.Sprintf("playRepeatedly(%q)", videoFile), nil); err != nil {
		return err
	}
	if err := seekVideoRepeatedly(ctx, conn, numSeeks); err != nil {
		return err
	}
	return nil
}

// colorDistance returns the maximum absolute difference between each component of a and b.
// Both a and b are assumed to be RGBA colors.
func colorDistance(a, b color.Color) int {
	aR, aG, aB, aA := a.RGBA()
	bR, bG, bB, bA := b.RGBA()
	abs := func(a int) int {
		if a < 0 {
			return -a
		}
		return a
	}
	max := func(nums ...int) int {
		m := 0
		for _, n := range nums {
			if n > m {
				m = n
			}
		}
		return m
	}
	// Interestingly, the RGBA method returns components in the range [0, 65535] (see
	// https://blog.golang.org/image). Therefore, we must shift them to the right by 8
	// so that they are in the more typical [0, 255] range.
	return max(abs(int(aR>>8)-int(bR>>8)),
		abs(int(aG>>8)-int(bG>>8)),
		abs(int(aB>>8)-int(bB>>8)),
		abs(int(aA>>8)-int(bA>>8)))
}

// TestPlay checks that the video file named filename can be played using Chrome.
// videotype represents a type of a given video. If it is MSEVideo, filename is a name
// of MPD file.
// If mode is VerifyHWAcceleratorUsed, this function also checks if hardware accelerator was used.
func TestPlay(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	filename string, videotype VideoType, mode VerifyHWAcceleratorMode) error {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		return err
	}
	defer vl.Close()

	if err := audio.Mute(ctx); err != nil {
		return err
	}
	defer audio.Unmute(ctx)

	var chromeMediaInternalsConn *chrome.Conn
	if mode != NoVerifyHWAcceleratorUsed {
		chromeMediaInternalsConn, err = decode.OpenChromeMediaInternalsPageAndInjectJS(ctx, cr, s.DataPath("chrome_media_internals_utils.js"))
		if err != nil {
			return errors.Wrap(err, "failed to open chrome://media-internals")
		}
		defer chromeMediaInternalsConn.Close()
		defer chromeMediaInternalsConn.CloseTarget(ctx)
	}

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	var playErr error
	var url string
	switch videotype {
	case NormalVideo:
		url = server.URL + "/video.html"
		playErr = playVideo(ctx, cr, filename, url)
	case MSEVideo:
		url = server.URL + "/shaka.html"
		playErr = playMSEVideo(ctx, cr, filename, url)
	}
	if playErr != nil {
		return errors.Wrapf(err, "failed to play %v (%v): %v", filename, url, playErr)
	}

	if mode == NoVerifyHWAcceleratorUsed {
		// Early return ig no verification is needed.
		return nil
	}

	usesPlatformVideoDecoder, err := decode.URLUsesPlatformVideoDecoder(ctx, chromeMediaInternalsConn, server.URL)
	if err != nil {
		return errors.Wrap(err, "failed to parse chrome:media-internals")
	}
	s.Log("usesPlatformVideoDecoder? ", usesPlatformVideoDecoder)

	if mode == VerifyHWAcceleratorUsed && !usesPlatformVideoDecoder {
		return errors.New("video decode acceleration was not used when it was expected to")
	}
	if mode == VerifyNoHWAcceleratorUsed && usesPlatformVideoDecoder {
		return errors.New("software decoding was not used when it was expected to")
	}
	return nil
}

// TestSeek checks that the video file named filename can be seeked around.
// It will play the video and seek randomly into it numSeeks times.
func TestSeek(ctx context.Context, httpHandler http.Handler, cr *chrome.Chrome, filename string, numSeeks int) error {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		return err
	}
	defer vl.Close()

	server := httptest.NewServer(httpHandler)
	defer server.Close()

	if err := playSeekVideo(ctx, cr, filename, server.URL, numSeeks); err != nil {
		return errors.Wrapf(err, "failed to play %v (%v): %v", filename, server.URL, err)
	}
	return nil
}

// TestPlayAndScreenshot plays the filename video, switches it to full
// screen mode, takes a screenshot and analyzes the resulting image to
// sample the colors of a few interesting points and compare them against
// expectations.
//
// Caveat: this test does not disable night light. Night light doesn't
// seem to affect the output of the screenshot tool, but this might
// not hold in the future in case we decide to apply night light at
// compositing time if the hardware does not support the color
// transform matrix.
func TestPlayAndScreenshot(ctx context.Context, s *testing.State, cr *chrome.Chrome, filename string) error {
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	url := path.Join(server.URL, "video.html")
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return errors.Wrapf(err, "failed to open %v", url)
	}
	defer conn.Close()

	// Make the video go to full screen mode by pressing 'f': requestFullScreen() needs a user gesture.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize the keyboard writer")
	}
	if err := ew.Type(ctx, "f"); err != nil {
		return errors.Wrap(err, "failed to inject the 'f' key")
	}

	// Start playing the video indefinitely.
	if err := conn.Eval(ctx, fmt.Sprintf("playRepeatedly(%q)", s.Param().(string)), nil); err != nil {
		return errors.Wrapf(err, "failed to play %v", filename)
	}

	// TODO(andrescj): this sleep is here to wait prior to taking the screenshot to make sure the video
	// is on the screen and to let the "Press Esc to exit full screen" message disappear. This is to
	// make sure the video is the only thing on the screen and thus minimize the excuses Chrome would
	// have to not promote it to a HW overlay. Poll instead for two conditions:
	// 1) The screenshot is correct (i.e., do the checks below), and
	// 2) There is a HW overlay.
	if err := testing.Sleep(ctx, delayToScreenshot); err != nil {
		return errors.Wrap(err, "failed to sleep prior to taking screenshot")
	}
	sshotPath := filepath.Join(s.OutDir(), "screenshot.png")
	if err := screenshot.Capture(ctx, sshotPath); err != nil {
		return errors.Wrap(err, "failed to capture screen")
	}

	// Decode the screenshot and rotate it if necessary to make later steps easier.
	f, err := os.Open(sshotPath)
	if err != nil {
		return errors.Wrapf(err, "failed to open %v", sshotPath)
	}
	img, _, err := image.Decode(f)
	// Close the file now because we might open it for writing again later.
	if err := f.Close(); err != nil {
		return errors.Wrapf(err, "failed to close %v", sshotPath)
	}
	if err != nil {
		return errors.Wrapf(err, "could not decode %v", sshotPath)
	}
	if img.Bounds().Dx() < img.Bounds().Dy() {
		s.Log("The screen is rotated; rotating the screenshot")
		rotImg := image.NewRGBA(image.Rectangle{image.Point{}, image.Point{img.Bounds().Max.Y, img.Bounds().Max.X}})
		for dstY := 0; dstY < rotImg.Bounds().Dy(); dstY++ {
			for dstX := 0; dstX < rotImg.Bounds().Dx(); dstX++ {
				srcColor := img.At(dstY, img.Bounds().Dy()-1-dstX)
				rotImg.Set(dstX, dstY, srcColor)
			}
		}
		f, err := os.Create(sshotPath)
		if err != nil {
			return errors.Wrapf(err, "could not create the rotated screenshot (%v)", sshotPath)
		}
		defer f.Close()
		if err := png.Encode(f, rotImg); err != nil {
			return errors.Wrapf(err, "could not encode the rotated screenshot (%v)", sshotPath)
		}
		img = rotImg
	}

	// Find the top and bottom of the video, i.e., exclude the black strips on top and bottom. Note
	// the video colors are chosen such that none of the RGB components are 0. We assume symmetry, so the
	// bottom is calculated based on the value of the top instead of using a loop (this is because the
	// bottom of the video can acceptably bleed into the bottom black strip and we want to ignore that).
	// No black strips are expected on the left or right.
	top := 0
	for ; top < img.Bounds().Dy(); top++ {
		if r, _, _, _ := img.At(0, top).RGBA(); r != 0 {
			break
		}
	}
	if top >= img.Bounds().Dy() {
		return errors.New("could not find the top of the video")
	}
	bottom := img.Bounds().Dy() - 1 - top
	left := 0
	right := img.Bounds().Dx() - 1

	// Calculate the coordinates of the centers of the four rectangles in the video.
	x25 := left + (right-left)/4
	y25 := top + (bottom-top)/4
	x75 := left + 3*(right-left)/4
	y75 := top + 3*(bottom-top)/4

	// These are the expectations on the four rectangles of the test video:
	//
	// - CornerX and CornerY are the coordinates of a corner of the video plus some padding. The padding
	//   is used to avoid testing exactly at the edges of the video where we might be subject to some
	//   color bleeding artifacts that are still acceptable.
	//
	// - CenterX, CenterY are the coordinates of the center of the rectangle.
	//
	// - Color is the expected color for the entire rectangle (this includes the corner and the centers).
	colors := map[string]struct {
		CornerX, CornerY int
		CenterX, CenterY int
		Color            color.Color
	}{
		"top-left":     {left + 2, top + 2, x25, y25, color.RGBA{128, 64, 32, 255}},
		"top-right":    {right - 2, top + 2, x75, y25, color.RGBA{32, 128, 64, 255}},
		"bottom-right": {right - 2, bottom - 2, x75, y75, color.RGBA{64, 32, 128, 255}},
		"bottom-left":  {left + 2, bottom - 2, x25, y75, color.RGBA{128, 32, 64, 255}},
	}

	const tolerance = 2

	// Check the colors of the centers of the four rectangles in the video.
	for k, v := range colors {
		actualColor := img.At(v.CenterX, v.CenterY)
		if colorDistance(actualColor, v.Color) > tolerance {
			return errors.Errorf("at the center of the %s rectangle (%d, %d): expected RGBA = %v; got RGBA = %v", k, v.CenterX, v.CenterY, v.Color, actualColor)
		}
	}

	// Check the color of the four corners of the video.
	for k, v := range colors {
		actualColor := img.At(v.CornerX, v.CornerY)
		if colorDistance(actualColor, v.Color) > tolerance {
			return errors.Errorf("at the %s corner (%d, %d): expected RGBA = %v; got RGBA = %v", k, v.CornerX, v.CornerY, v.Color, actualColor)
		}
	}
	return nil
}
