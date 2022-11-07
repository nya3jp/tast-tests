// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hdcputils

import (
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/media/imgcmp"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// ShakaPlayer represents connection to the shaka player web page.
type ShakaPlayer struct {
	// Conn represents a connection to a web content view, e.g. a tab.
	Conn *chrome.Conn
	// Tconn represents connection to the Tast test extension's background page.
	Tconn *chrome.TestConn
	// Cr represents Chrome login instance.
	Cr *chrome.Chrome
}

const (
	shakaPlayer = "https://integration.uat.widevine.com/player?autoplay=true&contentUrl=%s&proxyServerUrl=%s"
	hwSecureAll = "HW_SECURE_ALL"
)

var rdpPollOpts = &testing.PollOptions{Interval: time.Second, Timeout: 15 * time.Second}

// LaunchShakaPlayer launches shaka player with proxy and content URL.
func LaunchShakaPlayer(ctx context.Context, cr *chrome.Chrome, contentURL, proxyURL string) (*ShakaPlayer, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to test API")
	}
	// Create shaka player URL from content and proxy URL.
	webURL := fmt.Sprintf(shakaPlayer, contentURL, proxyURL)
	plyrConn, err := cr.NewConn(ctx, webURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open shaka player")
	}
	return &ShakaPlayer{Conn: plyrConn, Tconn: tconn, Cr: cr}, nil
}

// SelectVideoRobustness selects specified video robustness name in the drop down list.
func (s *ShakaPlayer) SelectVideoRobustness(ctx context.Context, name string) error {
	ui := uiauto.New(s.Tconn).WithPollOpts(*rdpPollOpts)
	if err := ui.LeftClick(nodewith.Name("Settings").Role(role.Button))(ctx); err != nil {
		return errors.Wrap(err, "failed to click on settings button")
	}
	section := nodewith.Role(role.PopUpButton).Ancestor(nodewith.Role(role.Section).First()).First()
	option := nodewith.Name(name).Role(role.ListBoxOption).First()
	if err := ui.LeftClickUntil(section, ui.WithTimeout(5*time.Second).WaitUntilExists(option))(ctx); err != nil {
		return errors.Wrap(err, "failed to click on videoRobustness section")
	}
	if err := ui.LeftClick(option)(ctx); err != nil {
		return errors.Wrapf(err, "failed to select option: %s", name)
	}
	// Close the settings popup window.
	if err := ui.LeftClick(nodewith.Name("CLOSE").Role(role.Button))(ctx); err != nil {
		return errors.Wrap(err, "failed to close settings popup window")
	}
	return nil
}

// PlayVideo click on video play button.
func (s *ShakaPlayer) PlayVideo(ctx context.Context) error {
	ui := uiauto.New(s.Tconn).WithPollOpts(*rdpPollOpts)
	play := nodewith.Name("Play").Role(role.Button).First()
	if err := ui.LeftClick(play)(ctx); err != nil {
		return errors.Wrap(err, "failed to click on play button")
	}
	// Check if any video player error.
	if err := s.CheckPlayerError(ctx); err != nil {
		return errors.Wrap(err, "player error found")
	}
	return nil
}

// CheckPlayerError checks if any player error pop window found.
func (s *ShakaPlayer) CheckPlayerError(ctx context.Context) error {
	ui := uiauto.New(s.Tconn).WithPollOpts(*rdpPollOpts)
	if err := ui.Exists(nodewith.Name("Player Error").Role(role.Heading))(ctx); err == nil {
		return errors.Wrap(err, "player error found")
	}
	return nil
}

// VerifyVideoPlayWithDuration verifies whether video is playing or not,
// it returns error if any player error or video current time is
// not reached provided duration or audio is not routing.
func (s *ShakaPlayer) VerifyVideoPlayWithDuration(ctx context.Context, duration int, cras *audio.Cras, expectedAudioNode string) error {
	totalSecBefore, err := s.currentTime(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to fetch video before time")
	}

	// Verify video play for the provided duration.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Check if any player error found for the specified video duration.
		totalSecAfter, err := s.currentTime(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to fetch video before time")
		}
		// Get current audio output device info.
		deviceName, deviceType, err := cras.SelectedOutputDevice(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get the selected audio device")
		}
		if deviceType != expectedAudioNode {
			return errors.Wrapf(err, "failed to set the audio node type: got %q; want %q", deviceType, expectedAudioNode)
		}

		// Verify audio routing on specified Speaker.
		if err := VerifyFirstRunningDevice(ctx, cras, deviceName); err != nil {
			return errors.Wrapf(err, "failed to route audio to %s", deviceName)
		}

		diffSec := totalSecAfter - totalSecBefore
		if diffSec < duration {
			return errors.Errorf("failed to play video until specified duration %d sec", duration)
		}
		return nil
	}, &testing.PollOptions{Timeout: 3 * time.Minute, Interval: 1 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to verify videoplay")
	}
	return nil
}

// FullScreenEntryExit func switches video between fullscreen and default screen.
func (s *ShakaPlayer) FullScreenEntryExit(ctx context.Context, iteration int) error {
	ui := uiauto.New(s.Tconn).WithPollOpts(*rdpPollOpts)
	fullScreen := nodewith.Name("Full screen").Role(role.Button)
	exitFullScreen := nodewith.Name("Exit full screen").Role(role.Button)
	for i := 1; i <= iteration; i++ {
		testing.ContextLogf(ctx, "Switching video between fullscreen and default screen iteration: %d/%d", i, iteration)
		if err := ui.MouseMoveTo(nodewith.ClassName("shaka-current-time"), 500*time.Millisecond)(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to move mouse to video seekbar")
		}
		// Enter video in fullscreen.
		if err := ui.LeftClick(fullScreen)(ctx); err != nil {
			return errors.Wrap(err, "failed to enter video in fullscreen")
		}
		// Exit video from fullscreen.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := ui.LeftClick(exitFullScreen)(ctx); err != nil {
				return errors.Wrap(err, "failed to exit video from fullscreen")
			}
			return nil
		}, &testing.PollOptions{Timeout: 3 * time.Second, Interval: 100 * time.Millisecond}); err != nil {
			return errors.Wrap(err, "failed to exit video from full screen")
		}
		// Check if any player error found for the specified video duration.
		if err := s.CheckPlayerError(ctx); err != nil {
			return errors.Wrap(err, "video player error found")
		}
		// Play video in default screen for at least 1.5 seconds.
		if err := testing.Sleep(ctx, 1500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to sleep")
		}
	}
	return nil
}

// VerifyVideoBlankScreen func verifies in the area where video is playing should be blank in screenshot.
func (s *ShakaPlayer) VerifyVideoBlankScreen(ctx context.Context, saveDir string, extDisplay bool) error {
	// Get Display Scale Factor to use it to convert bounds in dip to pixels.
	info, err := display.GetPrimaryInfo(ctx, s.Tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the primary display info")
	}
	mode, err := info.GetSelectedMode()
	if err != nil {
		return errors.Wrap(err, "failed to get the selected display mode of the primary display")
	}
	deviceScaleFactor := mode.DeviceScaleFactor
	videoPreview := nodewith.ClassName("shaka-video-container").Role(role.GenericContainer)
	// Capture screenshot on display, if extDisplay value true on external display else internal display.
	var videoImg image.Image
	if extDisplay {
		info, err := display.GetInfo(ctx, s.Tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get external display info")
		}
		windows, err := ash.GetAllWindows(ctx, s.Tconn)
		if err != nil {
			return errors.Wrap(err, "failed to obtain all windows")
		}
		if err := ash.SetWindowStateAndWait(ctx, s.Tconn, windows[0].ID, ash.WindowStateMaximized); err != nil {
			return errors.Wrap(err, "failed to maximize browser window")
		}
		videoImg, err = s.CaptureChromeForDisplay(ctx, info[1].ID)
		if err != nil {
			return errors.Wrap(err, "failed to get screenshot on external display")
		}
	} else {
		videoImg, err = s.GrapImgNodeScreenshot(ctx, videoPreview, deviceScaleFactor)
		if err != nil {
			return errors.Wrap(err, "failed to grab screenshot")
		}
	}

	// Verify that the video image is the (black image).
	// The image is now cropped to be a rectangle (filled with ~90% black).
	blackColor := color.RGBA{0, 0, 0, 255}
	prcnt := GetColorPercentage(videoImg, blackColor)
	threshold := 95
	if extDisplay {
		threshold = 37
	}
	if prcnt < threshold {
		return errors.Errorf("failed to verify video blank screen: Black pixels percentage: %d", prcnt)
	}
	return nil
}

// GetVideoCurrentTime returns video playing current time.
func (s *ShakaPlayer) GetVideoCurrentTime(ctx context.Context) (string, error) {
	if err := s.ShowVideoSeekbar(ctx); err != nil {
		return "", errors.Wrap(err, "failed to show video seekbar")
	}
	// Get video current time.
	var currentTime string
	if err := s.Conn.Eval(ctx, `document.getElementsByClassName("shaka-current-time")[0].innerText`, &currentTime); err != nil {
		return "", errors.Wrap(err, "failed to get video current time")
	}
	return currentTime, nil
}

// ShowVideoSeekbar hovers mouse on video seek bar to get updated current time.
func (s *ShakaPlayer) ShowVideoSeekbar(ctx context.Context) error {
	ui := uiauto.New(s.Tconn).WithPollOpts(*rdpPollOpts)
	for _, class := range []string{"shaka-current-time", "shaka-ad-markers"} {
		if err := ui.MouseMoveTo(nodewith.ClassName(class), 500*time.Millisecond)(ctx); err != nil {
			return errors.Wrap(err, "failed to move mouse to video seekbar")
		}
	}
	return nil
}

// GrapImgNodeScreenshot grabs screenshot on specific element
func (s *ShakaPlayer) GrapImgNodeScreenshot(ctx context.Context, node *nodewith.Finder, deviceScaleFactor float64) (image.Image, error) {
	ui := uiauto.New(s.Tconn)
	// Determine the bounds of node.
	loc, err := ui.Location(ctx, node)
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine the bounds of the image node")
	}
	rect := coords.ConvertBoundsFromDPToPX(*loc, deviceScaleFactor)
	return screenshot.GrabAndCropScreenshot(ctx, s.Cr, rect)
}

// GetColorPercentage returns color percentage of specified image.
func GetColorPercentage(img image.Image, clr color.RGBA) int {
	sim := imgcmp.CountPixelsWithDiff(img, clr, 60)
	bounds := img.Bounds()
	total := (bounds.Max.Y - bounds.Min.Y) * (bounds.Max.X - bounds.Min.X)
	prcnt := sim * 100 / total
	return prcnt
}

// CaptureChromeForDisplay takes a screenshot for a given displayID and return image object.
func (s *ShakaPlayer) CaptureChromeForDisplay(ctx context.Context, displayID string) (image.Image, error) {
	var base64PNG string
	if err := s.Tconn.Call(ctx, &base64PNG, "tast.promisify(chrome.autotestPrivate.takeScreenshotForDisplay)", displayID); err != nil {
		return nil, errors.Wrap(err, "failed to take screenshot")
	}
	sr := strings.NewReader(base64PNG)
	img, _, err := image.Decode(base64.NewDecoder(base64.StdEncoding, sr))
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode image")
	}
	return img, nil
}

// VerifyFirstRunningDevice will check for audio routing device status.
func VerifyFirstRunningDevice(ctx context.Context, cras *audio.Cras, expectedAudioNode string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
		if err != nil {
			return errors.Wrap(err, "failed to detect running output device")
		}
		if expectedAudioNode != devName {
			return errors.Wrapf(err, "failed to route the audio through expected audio node: got %q; want %q", devName, expectedAudioNode)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// currentTime returns current time of playing video.
func (s *ShakaPlayer) currentTime(ctx context.Context) (int, error) {
	currentTime, err := s.GetVideoCurrentTime(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get video current time")
	}
	timeBefore := strings.Split(strings.Split(currentTime, " ")[0], ":")
	m1, err := strconv.Atoi(strings.TrimSpace(timeBefore[0]))
	if err != nil {
		return 0, errors.Wrap(err, "failed to convert string to integer")
	}
	s1, err := strconv.Atoi(strings.TrimSpace(timeBefore[1]))
	if err != nil {
		return 0, errors.Wrap(err, "failed to convert string to integer")
	}
	timeInSecs := m1*60 + s1
	return timeInSecs, nil
}
