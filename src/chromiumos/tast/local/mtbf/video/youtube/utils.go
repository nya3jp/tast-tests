// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package youtube provides API to control a YouTube webpage
// through emulating user actions. (ex: clicking)
package youtube

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
	"chromiumos/tast/local/mtbf/dom"
	"chromiumos/tast/testing"
)

// VideoFrame is youtube video playing pixel.
type VideoFrame struct {
	X int
	Y int
}

const (
	// VideoPlayer represents the main <video> node in youtube page.
	VideoPlayer = "#movie_player > div.html5-video-container > video"
)

// tryToSkipAds checks youtube has ads or not, and will try to skip ads.
func tryToSkipAds(ctx context.Context, conn *chrome.Conn) error {
	// adSelector represents the skip ad node in youtube page.
	const adSelector = ".ytp-ad-skip-button-container"
	hasAds, err := dom.IsElementExists(ctx, conn, adSelector)
	if err != nil {
		return err
	}
	if hasAds {
		err = dom.ClickElement(ctx, conn, adSelector)
		if err != nil {
			return err
		}
		testing.Sleep(ctx, 1*time.Second)
	}
	return nil
}

// PlayVideo triggers VideoPlayer.play().
func PlayVideo(ctx context.Context, conn *chrome.Conn) error {
	tryToSkipAds(ctx, conn)
	if err := dom.PlayElement(ctx, conn, VideoPlayer); err != nil {
		return mtbferrors.New(mtbferrors.ChromeExeJs, err, "OpenAndPlayVideo")
	}
	return nil
}

// PauseVideo triggers VideoPlayer.pause().
func PauseVideo(ctx context.Context, conn *chrome.Conn) error {
	tryToSkipAds(ctx, conn)
	if err := dom.PauseElement(ctx, conn, VideoPlayer); err != nil {
		return mtbferrors.New(mtbferrors.VideoPauseFailed, err, "Youtube")
	}
	return nil
}

// ToggleFullScreen simulates keyboard input "F".
func ToggleFullScreen(ctx context.Context, conn *chrome.Conn) error {
	dom.WaitForElementBeingVisible(ctx, conn, VideoPlayer)
	if err := conn.Exec(ctx, dom.Query(VideoPlayer)+".focus()"); err != nil {
		return mtbferrors.New(mtbferrors.VideoEnterFullSc, err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return mtbferrors.New(mtbferrors.VideoEnterFullSc, err)
	}
	defer kb.Close()

	kb.Accel(ctx, "F")
	return nil
}

// OpenVideoSettings selects setting button and clicks it.
func OpenVideoSettings(ctx context.Context, conn *chrome.Conn) error {
	tryToSkipAds(ctx, conn)
	const menuButton = "#movie_player > div.ytp-chrome-bottom > div.ytp-chrome-controls > div.ytp-right-controls > button.ytp-button.ytp-settings-button"
	return dom.WaitAndClick(ctx, conn, menuButton)
}

// Quality values are regex string for matching quality (<select>) options.
var Quality = map[string]string{
	"4k":    "2160p",
	"2k":    "1440p",
	"1080p": "1080p",
	"720p":  "720p",
	"480p":  "480p",
	"360p":  "360p",
	"240p":  "240p",
}

// ChangeQuality changes the quality options.
func ChangeQuality(ctx context.Context, conn *chrome.Conn, quality string) (err error) {
	tryToSkipAds(ctx, conn)

	if err := OpenVideoSettings(ctx, conn); err != nil {
		return mtbferrors.New(mtbferrors.VideoOpenSettings, err)
	}
	testing.Sleep(ctx, 1*time.Second)
	const (
		buttonSelector      = ".ytp-popup.ytp-settings-menu > .ytp-panel > .ytp-panel-menu > .ytp-menuitem"
		buttonArrowFunction = "node => node.querySelector('.ytp-menuitem-label').innerText === 'Quality'"
	)
	if err := dom.WaitAndClick(ctx, conn, dom.AdvanceFindQuery(buttonSelector, buttonArrowFunction)); err != nil {
		return mtbferrors.New(mtbferrors.VideoWaitAndClick, err, buttonSelector)
	}

	// Wait for animation.
	testing.Sleep(ctx, 1*time.Second)
	const (
		qualitySelector      = ".ytp-popup.ytp-settings-menu > .ytp-panel.ytp-quality-menu > .ytp-panel-menu > .ytp-menuitem"
		qualityArrowFunction = "item => item.innerText.match(/%s/)"
	)
	if err := dom.WaitAndClick(ctx, conn, dom.AdvanceFindQuery(qualitySelector, fmt.Sprintf(qualityArrowFunction, quality))); err != nil {
		return mtbferrors.New(mtbferrors.VideoWaitAndClick, err, qualitySelector)
	}
	// Wait for video to change quality...
	if err := WaitForReadyState(ctx, conn); err != nil {
		return mtbferrors.New(mtbferrors.VideoReadyStatePoll, err)
	}
	return nil
}

// OpenStatsForNerds clicks on an option in context menu of VideoPlayer to display statistic information dialog.
func OpenStatsForNerds(ctx context.Context, conn *chrome.Conn) (err error) {
	tryToSkipAds(ctx, conn)

	const videoPlayerContainer = "#movie_player"
	if err = dom.RightClickElement(ctx, conn, videoPlayerContainer); err != nil {
		return mtbferrors.New(mtbferrors.VideoStatsNerd, err)
	}

	const (
		selector      = "body > div.ytp-popup.ytp-contextmenu > div > div > .ytp-menuitem"
		arrowFunction = "node => node.querySelector('.ytp-menuitem-label').innerText === 'Stats for nerds'"
	)
	if err = dom.WaitAndClick(ctx, conn, dom.AdvanceFindQuery(selector, arrowFunction)); err != nil {
		return mtbferrors.New(mtbferrors.VideoStatsNerd, err)
	}

	return nil
}

// CurrentTime returns VideoPlayer.currentTime.
func CurrentTime(ctx context.Context, conn *chrome.Conn) (time float64, err error) {
	tryToSkipAds(ctx, conn)

	time, err = dom.GetElementCurrentTime(ctx, conn, VideoPlayer)
	return
}

// GetFrameDropsFromStatsForNerds will return frame drops value by executing javascript.
func GetFrameDropsFromStatsForNerds(ctx context.Context, conn *chrome.Conn) (framedrops int, err error) {
	tryToSkipAds(ctx, conn)
	const (
		javascript    = "parseInt(%s.querySelector('span').innerText.match(/\\ \\d*\\ /)[0])"
		selector      = "#movie_player > div.html5-video-info-panel > div > div"
		arrowFunction = "node => node.innerText.indexOf('Frames') >= 0"
	)
	err = conn.Eval(ctx, fmt.Sprintf(javascript, dom.AdvanceFindQuery(selector, arrowFunction)), &framedrops)
	return
}

// getResolutionByText returns video frame x and y by text.
func getResolutionByText(ctx context.Context, conn *chrome.Conn, text string) (videoFrame VideoFrame, err error) {
	const (
		javascript = "%s.querySelector('span').innerText.match(/\\d*x\\d*/)[0].split('x').map(val => parseInt(val))"
		selector   = "#movie_player > div.html5-video-info-panel > div > div"
	)
	arrowFunction := fmt.Sprintf("node => node.innerText.indexOf('%s') >= 0", text)
	var intArray [2]int
	err = conn.Eval(ctx, fmt.Sprintf(javascript, dom.AdvanceFindQuery(selector, arrowFunction)), &intArray)
	if err != nil {
		return
	}
	videoFrame.X = intArray[0]
	videoFrame.Y = intArray[1]
	return
}

// getFramePerSecondFromStatsForNerds will return frame per second by executing javascript.
func getFramePerSecondFromStatsForNerds(ctx context.Context, conn *chrome.Conn) (fps int, err error) {
	const (
		javascript    = "parseInt(%s.querySelector('span').innerText.match(/\\d*x\\d*@\\d*/)[0].split('@')[1])"
		selector      = "#movie_player > div.html5-video-info-panel > div > div"
		arrowFunction = "node => node.innerText.indexOf('Optimal Res') >= 0"
	)
	err = conn.Eval(ctx, fmt.Sprintf(javascript, dom.AdvanceFindQuery(selector, arrowFunction)), &fps)
	if err != nil {
		return 0, mtbferrors.New(mtbferrors.VideoFailFramesPerSeconds, err)
	}
	return
}

// GetViewportFromStatsForNerds will return current video view pixel.
func GetViewportFromStatsForNerds(ctx context.Context, conn *chrome.Conn) (videoFrame VideoFrame, err error) {
	tryToSkipAds(ctx, conn)
	return getResolutionByText(ctx, conn, "Frames")
}

// GetCurrentResolutionFromStatsForNerds will return current video resolution.
func GetCurrentResolutionFromStatsForNerds(ctx context.Context, conn *chrome.Conn) (videoFrame VideoFrame, err error) {
	tryToSkipAds(ctx, conn)
	videoFrame, err = getResolutionByText(ctx, conn, "Optimal Res")
	if err != nil {
		return videoFrame, mtbferrors.New(mtbferrors.VideoGetRatio, err)
	}
	return videoFrame, nil
}

// OpenAndPlayVideo opens a new connection and plays video.
func OpenAndPlayVideo(ctx context.Context, cr *chrome.Chrome, url string) (*chrome.Conn, error) {
	conn, mtbferr := mtbfchrome.NewConn(ctx, cr, url)
	if mtbferr != nil {
		return nil, mtbferr
	}

	testing.Sleep(ctx, 3*time.Second)
	if mtbferr := PlayVideo(ctx, conn); mtbferr != nil {
		return nil, mtbferr
	}

	return conn, nil
}

// OpenAndPlayMultipleVideosInTabs creates multiple youtube videos in different tabs and play.
func OpenAndPlayMultipleVideosInTabs(ctx context.Context, cr *chrome.Chrome, urls []string) ([]*chrome.Conn, error) {
	conns := make([]*chrome.Conn, 0)

	for _, url := range urls {
		conn, err := OpenAndPlayVideo(ctx, cr, url)
		if err != nil {
			return nil, err
		}
		conns = append(conns, conn)
	}

	return conns, nil
}

// WaitForReadyState wait for ready state
func WaitForReadyState(ctx context.Context, conn *chrome.Conn) error {
	if err := dom.WaitForReadyState(ctx, conn, VideoPlayer, 10*time.Second, 100*time.Millisecond); err != nil {
		return mtbferrors.New(mtbferrors.VideoReadyStatePoll, err)
	}
	return nil
}

// Add1SecondForURL adds a start play time for one second of youtube url.
func Add1SecondForURL(url string) (youtube string) {
	youtube = url + "&t=1"
	return youtube
}
