// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package play provides common code for playing videos on Chrome.
package play

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/devtools"
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
	// DRMVideo represents a video requiring Digital Rights Management (DRM).
	DRMVideo
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
	// VerifyHWDRMUsed is a mode that verifies a video is played using a hardware
	// accelerator with HW DRM protection.
	VerifyHWDRMUsed
)

// This is how long we need to wait before taking a screenshot in the
// TestPlayAndScreenshot case. This is necessary to ensure the video is on the screen
// and to let the "Press Esc to exit full screen" message disappear.
const delayToScreenshot = 7 * time.Second

// MSEDataFiles returns a list of required files for tests that play MSE videos.
func MSEDataFiles() []string {
	return []string{
		"shaka.html",
		"third_party/shaka-player/shaka-player.compiled.debug.js",
		"third_party/shaka-player/shaka-player.compiled.debug.map",
	}
}

// DRMDataFiles returns a list of required files for tests that play DRM videos.
func DRMDataFiles() []string {
	return []string{
		"shaka_drm.html",
		"third_party/shaka-player/shaka-player.compiled.debug.js",
		"third_party/shaka-player/shaka-player.compiled.debug.map",
	}
}

// loadPage opens a new tab to load the specified webpage.
// Note that if err != nil, conn is nil.
func loadPage(ctx context.Context, cs ash.ConnSource, url string) (*chrome.Conn, error) {
	ctx, st := timing.Start(ctx, "load_page")
	defer st.End()

	conn, err := cs.NewConn(ctx, url)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open %v", url)
	}
	return conn, err
}

// playVideo invokes loadVideo(), plays a normal video in video.html, and checks if it has progress.
// videoFile is the file name which is played there.
// url is the URL of the video playback testing webpage.
func playVideo(ctx context.Context, cs ash.ConnSource, videoFile, url string, unmutePlayer bool) (bool, error) {
	ctx, st := timing.Start(ctx, "play_video")
	defer st.End()

	conn, err := loadPage(ctx, cs, url)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	observer, err := conn.GetMediaPropertiesChangedObserver(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to retrieve a media DevTools observer")
	}

	if err := conn.Call(ctx, nil, "playUntilEnd", videoFile, unmutePlayer); err != nil {
		return false, err
	}

	isPlatform, _, err := devtools.GetVideoDecoder(ctx, observer, url)
	return isPlatform, err
}

// playMSEVideo plays an MSE video stream via Shaka player, and checks its play progress.
// mpdFile is the name of MPD file for the video stream.
// url is the URL of the shaka player webpage.
func playMSEVideo(ctx context.Context, cs ash.ConnSource, mpdFile, url string) (bool, error) {
	ctx, st := timing.Start(ctx, "play_mse_video")
	defer st.End()

	conn, err := loadPage(ctx, cs, url)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	observer, err := conn.GetMediaPropertiesChangedObserver(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to retrieve a media DevTools observer")
	}

	if err := conn.Call(ctx, nil, "play_shaka", mpdFile); err != nil {
		return false, err
	}

	isPlatform, _, err := devtools.GetVideoDecoder(ctx, observer, url)
	return isPlatform, err
}

// playDRMVideo plays a DRM-protected MSE video stream via Shaka player, and
// checks its play progress. After it's done, it goes full screen and takes a
// screenshot and verifies the contents are all black.
// mpdFile is the name of MPD file for the video stream.cs ash.ConnSource,
// url is the URL of the shaka player webpage.
func playDRMVideo(ctx context.Context, s *testing.State, cs ash.ConnSource, cr *chrome.Chrome, mpdFile, url string) (bool, error) {
	ctx, st := timing.Start(ctx, "play_drm_video")
	defer st.End()

	conn, err := loadPage(ctx, cs, url)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	observer, err := conn.GetMediaPropertiesChangedObserver(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to retrieve a media DevTools observer")
	}

	if err := conn.Call(ctx, nil, "play_shaka_drm", mpdFile); err != nil {
		return false, err
	}

	// Now go full screen, take a screenshot and verify it's all black.

	// Make the video go to full screen mode by pressing 'f': requestFullScreen() needs a user gesture.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to initialize the keyboard writer")
	}
	defer ew.Close()
	if err := ew.Type(ctx, "f"); err != nil {
		return false, errors.Wrap(err, "failed to inject the 'f' key")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to connect to test API")
	}
	defer tconn.Close()

	if err := ash.WaitForFullScreen(ctx, tconn); err != nil {
		return false, errors.Wrap(err, "failed waiting for full screen")
	}

	// Take the screenshot, we don't need to wait because we are only verifying
	// that the vast majority is black, so things like 'hit Esc to exist full screen'
	// won't be an issue.
	im, err := screenshot.GrabScreenshot(ctx, cr)
	if err != nil {
		return false, errors.Wrap(err, "failed taking screenshot")
	}

	// Verify that over 95% of the image is solid black. This is true because for
	// HW DRM, you cannot actually screenshot the video and it will be replaced by
	// solid black in the compositor.
	color, ratio := colorcmp.DominantColor(im)
	if ratio < 0.95 || !colorcmp.ColorsMatch(color, colorcmp.RGB(0, 0, 0), 1) {
		return false, errors.Errorf("screenshot did not have solid black, instead got %v at ratio %0.2f",
			colorcmp.ColorStr(color), ratio)
	}

	return devtools.CheckHWDRMPipeline(ctx, observer, url)
}

// seekVideoRepeatedly seeks video numSeeks times.
func seekVideoRepeatedly(ctx context.Context, conn *chrome.Conn, numSeeks int) error {
	ctx, st := timing.Start(ctx, "seek_video_repeatly")
	defer st.End()
	prevFinishedSeeks := 0
	for i := 0; i < numSeeks; i++ {
		finishedSeeks := 0
		if err := conn.Call(ctx, &finishedSeeks, "randomSeek"); err != nil {
			// If the test times out, Call() might be interrupted and return
			// zero finishedSeeks, in that case used the last known good amount.
			if finishedSeeks == 0 {
				finishedSeeks = prevFinishedSeeks
			}
			return errors.Wrapf(err, "error while seeking, completed %d/%d seeks", finishedSeeks, numSeeks)
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
func playSeekVideo(ctx context.Context, cs ash.ConnSource, videoFile, baseURL string, numSeeks int) error {
	ctx, st := timing.Start(ctx, "play_seek_video")
	defer st.End()

	// Establish a connection to a video play page
	conn, err := loadPage(ctx, cs, baseURL+"/video.html")
	if err != nil {
		return err
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	if err := conn.Call(ctx, nil, "playRepeatedly", videoFile); err != nil {
		return err
	}
	if err := seekVideoRepeatedly(ctx, conn, numSeeks); err != nil {
		return err
	}
	return nil
}

// ColorDistance returns the maximum absolute difference between each component of a and b.
// Both a and b are assumed to be RGBA colors.
func ColorDistance(a, b color.Color) int {
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
	// Interestingly, the RGBA method returns components in the range [0, 0xFFFF] corresponding
	// to the 8-bit values multiplied by 0x101 (see https://blog.golang.org/image). Therefore,
	// we must shift them to the right by 8 so that they are in the more typical [0, 255] range.
	return max(abs(int(aR>>8)-int(bR>>8)),
		abs(int(aG>>8)-int(bG>>8)),
		abs(int(aB>>8)-int(bB>>8)),
		abs(int(aA>>8)-int(bA>>8)))
}

// ColorSamplingPointsForStillColorsVideo returns a map of points that are considered to be
// interesting in the rendering of the still-colors-*.mp4 test videos. The key in the map is
// a name for the corresponding point. There are two categories of points:
//
// - Outer corners: the four absolute corners of the video offset by 1 to ignore acceptable
//   color blending artifacts on the edges. However, the outer bottom-right is not offset
//   because we never expect blending artifacts there.
//
// - Inner corners: 4 stencils (one for each corner of the video). Each stencil is composed
//   of 4 sampling points arranged as a square. The expectation is that for each stencil, 3
//   of its points fall on the interior border of the test video while the remaining point
//   falls inside one of the color rectangles. This helps us detect undesired
//   stretching/shifting/rotation/mirroring. The naming convention for each point of a
//   stencil is as follows:
//
//     inner_Y_X_00: the corner of the stencil closest to the Y-X corner of the video.
//     inner_Y_X_01: the corner of the stencil that's in the interior X border of the video.
//     inner_Y_X_10: the corner of the stencil that's in the interior Y border of the video.
//     inner_Y_X_11: the only corner of the stencil that's not on the border strip.
//
//   For example, the top-right corner of the test video looks like this:
//
//     MMMMMMMMMMMMMMMM
//     MMMMMMMMMM2MMM0M
//     MMMMMMMMMMMMMMMM
//               3  M1M
//                  MMM
//
//   Where 'M' is the magenta interior border. So the names of each of the points 0, 1, 2, 3
//   are:
//
//     0: inner_top_right_00
//     1: inner_top_right_01
//     2: inner_top_right_10
//     3: inner_top_right_11
func ColorSamplingPointsForStillColorsVideo(videoW, videoH int) map[string]image.Point {
	outerCorners := map[string]image.Point{
		"outer_top_left":     {1, 1},
		"outer_top_right":    {(videoW - 1) - 1, 1},
		"outer_bottom_right": {videoW - 1, videoH - 1},
		"outer_bottom_left":  {1, (videoH - 1) - 1},
	}
	edgeOffset := 5
	stencilW := 5
	innerCorners := map[string]image.Point{
		"inner_top_left_00":     {edgeOffset, edgeOffset},
		"inner_top_left_01":     {edgeOffset, edgeOffset + stencilW},
		"inner_top_left_10":     {edgeOffset + stencilW, edgeOffset},
		"inner_top_left_11":     {edgeOffset + stencilW, edgeOffset + stencilW},
		"inner_top_right_00":    {(videoW - 1) - edgeOffset, edgeOffset},
		"inner_top_right_01":    {(videoW - 1) - edgeOffset, edgeOffset + stencilW},
		"inner_top_right_10":    {(videoW - 1) - edgeOffset - stencilW, edgeOffset},
		"inner_top_right_11":    {(videoW - 1) - edgeOffset - stencilW, edgeOffset + stencilW},
		"inner_bottom_right_00": {(videoW - 1) - edgeOffset, (videoH - 1) - edgeOffset},
		"inner_bottom_right_01": {(videoW - 1) - edgeOffset, (videoH - 1) - edgeOffset - stencilW},
		"inner_bottom_right_10": {(videoW - 1) - edgeOffset - stencilW, (videoH - 1) - edgeOffset},
		"inner_bottom_right_11": {(videoW - 1) - edgeOffset - stencilW, (videoH - 1) - edgeOffset - stencilW},
		"inner_bottom_left_00":  {edgeOffset, (videoH - 1) - edgeOffset},
		"inner_bottom_left_01":  {edgeOffset, (videoH - 1) - edgeOffset - stencilW},
		"inner_bottom_left_10":  {edgeOffset + stencilW, (videoH - 1) - edgeOffset},
		"inner_bottom_left_11":  {edgeOffset + stencilW, (videoH - 1) - edgeOffset - stencilW},
	}
	samples := map[string]image.Point{}
	for k, v := range innerCorners {
		samples[k] = v
	}
	for k, v := range outerCorners {
		samples[k] = v
	}
	return samples
}

// isVideoPadding returns true iff c corresponds to the expected color of the padding that a
// video gets when in full screen so that it appears centered. This color is black within a
// certain tolerance.
func isVideoPadding(c color.Color) bool {
	black := color.RGBA{0, 0, 0, 255}
	// The tolerance was picked empirically. For example, on kukui, the first padding row below
	// the video has a color of (20, 1, 22, 255).
	tolerance := 25
	return ColorDistance(c, black) < tolerance
}

// TestPlay checks that the video file named filename can be played using Chrome.
// videotype represents a type of a given video. If it is MSEVideo, filename is a name
// of MPD file.
// If mode is VerifyHWAcceleratorUsed, this function also checks if hardware accelerator was used.
func TestPlay(ctx context.Context, s *testing.State, cs ash.ConnSource, cr *chrome.Chrome,
	filename string, videotype VideoType, mode VerifyHWAcceleratorMode, unmutePlayer bool) error {
	if unmutePlayer && videotype != NormalVideo {
		return errors.New("got unmutePlayer = true, expected false: unmutePlayer " +
			"is only implemented for videoType = NormalVideo")
	}
	vl, err := logging.NewVideoLogger()
	if err != nil {
		return err
	}
	defer vl.Close()

	if err := crastestclient.Mute(ctx); err != nil {
		return err
	}
	defer crastestclient.Unmute(ctx)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	var playErr error
	var url string
	usesPlatformVideoDecoder, isHwDrmPipeline := false, false
	switch videotype {
	case NormalVideo:
		url = server.URL + "/video.html"
		usesPlatformVideoDecoder, playErr = playVideo(ctx, cs, filename, url, unmutePlayer)
	case MSEVideo:
		url = server.URL + "/shaka.html"
		usesPlatformVideoDecoder, playErr = playMSEVideo(ctx, cs, filename, url)
	case DRMVideo:
		url = server.URL + "/shaka_drm.html"
		isHwDrmPipeline, playErr = playDRMVideo(ctx, s, cs, cr, filename, url)
	}
	if playErr != nil {
		return errors.Wrapf(err, "failed to play %v (%v): %v", filename, url, playErr)
	}

	if mode == NoVerifyHWAcceleratorUsed {
		// Early return when no verification is needed.
		return nil
	}

	if mode == VerifyHWAcceleratorUsed && !usesPlatformVideoDecoder {
		return errors.New("video decode acceleration was not used when it was expected to")
	}
	if mode == VerifyNoHWAcceleratorUsed && usesPlatformVideoDecoder {
		return errors.New("software decoding was not used when it was expected to")
	}
	if mode == VerifyHWDRMUsed && !isHwDrmPipeline {
		return errors.New("HW DRM video pipeline was not used when it was expected to")
	}
	return nil
}

// TestSeek checks that the video file named filename can be seeked around.
// It will play the video and seek randomly into it numSeeks times.
func TestSeek(ctx context.Context, httpHandler http.Handler, cs ash.ConnSource, filename string, numSeeks int) error {
	vl, err := logging.NewVideoLogger()
	if err != nil {
		return err
	}
	defer vl.Close()

	server := httptest.NewServer(httpHandler)
	defer server.Close()

	if err := playSeekVideo(ctx, cs, filename, server.URL, numSeeks); err != nil {
		return errors.Wrapf(err, "failed to play %v (%v): %v", filename, server.URL, err)
	}
	return nil
}

// TestPlayAndScreenshot plays the filename video, switches it to full
// screen mode, takes a screenshot and analyzes the resulting image to
// sample the colors of a few interesting points and compare them against
// expectations. The expectations are defined by refFilename which is a
// PNG file corresponding to the ideally rendered video frame in the absence
// of scaling or artifacts.
//
// Caveat: this test does not disable night light. Night light doesn't
// seem to affect the output of the screenshot tool, but this might
// not hold in the future in case we decide to apply night light at
// compositing time if the hardware does not support the color
// transform matrix.
func TestPlayAndScreenshot(ctx context.Context, s *testing.State, tconn *chrome.TestConn, cs ash.ConnSource, filename, refFilename string) error {
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	url := path.Join(server.URL, "video.html")
	conn, err := cs.NewConn(ctx, url)
	if err != nil {
		return errors.Wrapf(err, "failed to open %v", url)
	}
	defer conn.Close()

	// For consistency across test runs, ensure that the device is in landscape-primary orientation.
	if err = graphics.RotateDisplayToLandscapePrimary(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to set display to landscape-primary orientation")
	}

	// Make the video go to full screen mode by pressing 'f': requestFullScreen() needs a user gesture.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize the keyboard writer")
	}
	if err := ew.Type(ctx, "f"); err != nil {
		return errors.Wrap(err, "failed to inject the 'f' key")
	}

	// Start playing the video indefinitely.
	if err := conn.Call(ctx, nil, "playRepeatedly", filename); err != nil {
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
		s.Log("The screenshot is in portrait orientation; rotating it")
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

	// Find the bounds of the video by excluding the black strips on each side.
	xMiddle := img.Bounds().Dx() / 2
	yMiddle := img.Bounds().Dy() / 2
	top := 0
	for ; top < img.Bounds().Dy(); top++ {
		if !isVideoPadding(img.At(xMiddle, top)) {
			break
		}
	}
	bottom := img.Bounds().Dy() - 1
	for ; bottom >= 0; bottom-- {
		if !isVideoPadding(img.At(xMiddle, bottom)) {
			break
		}
	}
	if bottom <= top {
		return errors.New("could not find the top or bottom boundary of the video")
	}
	left := 0
	for ; left < img.Bounds().Dx(); left++ {
		if !isVideoPadding(img.At(left, yMiddle)) {
			break
		}
	}
	right := img.Bounds().Dx() - 1
	for ; right >= 0; right-- {
		if !isVideoPadding(img.At(right, yMiddle)) {
			break
		}
	}
	if right <= left {
		return errors.New("could not find the left or right boundary of the video")
	}
	s.Logf("Video bounds: (left, top) = (%d, %d); (right, bottom) = (%d, %d)",
		left, top, right, bottom)

	// Open the reference file to assert expectations on the screenshot later.
	refPath := s.DataPath(refFilename)
	f, err = os.Open(refPath)
	if err != nil {
		return errors.Wrapf(err, "failed to open %v", refPath)
	}
	defer f.Close()
	refImg, _, err := image.Decode(f)
	if err != nil {
		return errors.Wrapf(err, "could not decode %v", refPath)
	}
	videoW := refImg.Bounds().Dx()
	videoH := refImg.Bounds().Dy()

	// Measurement 1:
	// We'll sample a few interesting pixels and report the color distance with
	// respect to the reference image.
	samples := ColorSamplingPointsForStillColorsVideo(videoW, videoH)
	p := perf.NewValues()
	maxDistance := -1
	maxDistancePoint := ""
	for k, v := range samples {
		// First convert the coordinates from video space to screenshot space.
		videoX := v.X
		videoY := v.Y
		screenX := left + (right-left)*v.X/(videoW-1)
		screenY := top + (bottom-top)*v.Y/(videoH-1)

		// Then report the distance between the expected and actual colors at this location.
		expectedColor := refImg.At(videoX, videoY)
		actualColor := img.At(screenX, screenY)
		distance := ColorDistance(expectedColor, actualColor)
		if distance > maxDistance {
			maxDistance = distance
			maxDistancePoint = k
		}
		if distance != 0 {
			s.Logf("At %v (video space = (%d, %d), screen space = (%d, %d)): expected RGBA = %v; got RGBA = %v; distance = %d",
				k, videoX, videoY, screenX, screenY, expectedColor, actualColor, distance)
		}
		p.Set(perf.Metric{
			Name:      k,
			Unit:      "None",
			Direction: perf.SmallerIsBetter,
		}, float64(distance))
	}

	// The distance threshold was decided by analyzing the data reported above
	// across many devices. It should ideally be smaller, but for now, it seems we
	// have color space handling issues. Nonetheless, this threshold should be
	// enough for detecting major video rendering issues. Note that:
	//
	// 1) We still report the distances as perf values so we can continue to
	//    analyze and improve.
	// 2) We don't bother to report a total distance if this threshold is exceeded
	//    because it would just make email alerts very noisy.
	if maxDistance > 100 {
		p.Save(s.OutDir())
		return errors.Errorf("the color distance for %v = %d exceeds the threshold (100)", maxDistancePoint, maxDistance)
	}

	// Measurement 2:
	// We report an aggregate distance for the image: we go through all the pixels
	// in the screenshot video to add up all the distances and then normalize by
	// the number of pixels at the end.
	totalDistance := 0.0
	for row := top; row <= bottom; row++ {
		for col := left; col <= right; col++ {
			// First convert the coordinates from screenshot space to video space.
			videoX := (col - left) * (videoW - 1) / (right - left)
			videoY := (row - top) * (videoH - 1) / (bottom - top)
			expectedColor := refImg.At(videoX, videoY)
			actualColor := img.At(col, row)
			totalDistance += float64(ColorDistance(expectedColor, actualColor))
		}
	}
	totalDistance /= float64((right - left + 1) * (bottom - top + 1))
	s.Log("The total distance for the entire image is ", totalDistance)
	p.Set(perf.Metric{
		Name:      "total_distance",
		Unit:      "None",
		Direction: perf.SmallerIsBetter,
	}, totalDistance)
	p.Save(s.OutDir())

	return nil
}
