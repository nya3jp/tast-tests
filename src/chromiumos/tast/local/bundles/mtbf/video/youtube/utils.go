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

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cdputil/dom"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// VideoFrame is youtube video playing pixel.
type VideoFrame struct {
	X int
	Y int
}

// VideoPlayer represents the main <video> node in youtube page
const VideoPlayer = "#movie_player > div.html5-video-container > video"

// adSelector represents the skip ad node in youtube page.
const adSelector = ".ytp-ad-skip-button-container"

// isAds checks youtube has ads or not, and will try to skip ads.
func isAds(ctx context.Context, conn *chrome.Conn) error {
	testing.Sleep(ctx, 1*time.Second)
	isAds, err := dom.IsElementExists(ctx, conn, adSelector)
	if err != nil {
		return err
	}
	testing.Sleep(ctx, 1*time.Second)
	if isAds {
		err = dom.ClickElement(ctx, conn, adSelector)
		if err != nil {
			return err
		}
	}
	return nil
}

// PlayVideo triggers VideoPlayer.play()
func PlayVideo(ctx context.Context, conn *chrome.Conn) error {
	if err := isAds(ctx, conn); err != nil {
		return err
	}
	return dom.PlayElement(ctx, conn, VideoPlayer)
}

// PauseVideo triggers VideoPlayer.pause()
func PauseVideo(ctx context.Context, conn *chrome.Conn) error {
	if err := isAds(ctx, conn); err != nil {
		return err
	}
	return dom.PauseElement(ctx, conn, VideoPlayer)
}

// ToggleFullScreen simulates keyboard input "F"
func ToggleFullScreen(ctx context.Context, conn *chrome.Conn) error {
	dom.WaitForElementBeingVisiable(ctx, conn, VideoPlayer)
	if err := conn.Exec(ctx, dom.Query(VideoPlayer)+".focus()"); err != nil {
		return err
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return err
	}
	defer kb.Close()

	kb.Accel(ctx, "F")
	return nil
}

// OpenVideoSettings selects setting button and clicks it
func OpenVideoSettings(ctx context.Context, conn *chrome.Conn) error {
	const menuButton = "#movie_player > div.ytp-chrome-bottom > div.ytp-chrome-controls > div.ytp-right-controls > button.ytp-button.ytp-settings-button"
	return dom.WaitAndClick(ctx, conn, menuButton)
}

// Quality values are regex string for matching quality (<select>) options
var Quality = map[string]string{
	"4k":    "2160p",
	"2k":    "1440p",
	"1080p": "1080p",
	"720p":  "720p",
	"480p":  "480p",
	"360p":  "360p",
	"240p":  "240p",
}

// ChangeQuality changes the quality options
func ChangeQuality(ctx context.Context, conn *chrome.Conn, quality string) (err error) {
	if err := isAds(ctx, conn); err != nil {
		return err
	}
	if err = OpenVideoSettings(ctx, conn); err != nil {
		return
	}
	testing.Sleep(ctx, 1*time.Second)
	const (
		buttonSelector      = "#ytp-id-20 > .ytp-panel > .ytp-panel-menu > .ytp-menuitem"
		buttonArrowFunction = "node => node.querySelector('.ytp-menuitem-label').innerText === 'Quality'"
	)
	if err = dom.WaitAndClick(ctx, conn, dom.AdvanceFindQuery(buttonSelector, buttonArrowFunction)); err != nil {
		return
	}

	// Wait for animation
	if err = testing.Sleep(ctx, 1*time.Second); err != nil {
		return
	}
	const (
		qualitySelector      = "#ytp-id-20 > div.ytp-panel.ytp-quality-menu > div.ytp-panel-menu .ytp-menuitem"
		qualityArrowFunction = "item => item.innerText.match(/%s/)"
	)
	if err = dom.WaitAndClick(ctx, conn, dom.AdvanceFindQuery(qualitySelector, fmt.Sprintf(qualityArrowFunction, quality))); err != nil {
		return
	}

	return nil
}

// OpenStatsForNerd clicks on an option in context menu of VideoPlayer to display statistic infomation dialog
func OpenStatsForNerd(ctx context.Context, conn *chrome.Conn) (err error) {
	const videoPlayerContainer = "#movie_player"
	if err = dom.RightClickElement(ctx, conn, videoPlayerContainer); err != nil {
		return
	}

	const (
		selector      = "body > div.ytp-popup.ytp-contextmenu > div > div > .ytp-menuitem"
		arrowFunction = "node => node.querySelector('.ytp-menuitem-label').innerText === 'Stats for nerds'"
	)
	if err = dom.WaitAndClick(ctx, conn, dom.AdvanceFindQuery(selector, arrowFunction)); err != nil {
		return
	}

	return nil
}

// GetCurrentTime returns VideoPlayer.currentTime
func GetCurrentTime(ctx context.Context, conn *chrome.Conn) (time float64, err error) {
	time, err = dom.GetElementCurrentTime(ctx, conn, VideoPlayer)
	return
}

// GetFrameDropsFromStatsForNerd will return frame drops value by executing javascript.
func GetFrameDropsFromStatsForNerd(ctx context.Context, conn *chrome.Conn) (framedrops int, err error) {
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

// GetViewportFromStatsForNerd will return current video view pixel.
func GetViewportFromStatsForNerd(ctx context.Context, conn *chrome.Conn) (videoFrame VideoFrame, err error) {
	return getResolutionByText(ctx, conn, "Frames")
}

// GetCurrentResolutionFromStatsForNerd will return current video resolution.
func GetCurrentResolutionFromStatsForNerd(ctx context.Context, conn *chrome.Conn) (videoFrame VideoFrame, err error) {
	return getResolutionByText(ctx, conn, "Optimal Res")
}

// OpenAndPlayVideo opens a new connection and plays video
func OpenAndPlayVideo(ctx context.Context, cr *chrome.Chrome, url string) (*chrome.Conn, error) {
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return nil, err
	}

	if err := PlayVideo(ctx, conn); err != nil {
		return nil, err
	}

	return conn, nil
}

// OpenAndPlayMultipleVideosInTabs creates multiple youtube videos in different tabs and playing
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

// Add1SecondForURL adds a start play time for one second of youtube url.
func Add1SecondForURL(url string) (youtube string) {
	youtube = url + "&t=1"
	return youtube
}
