// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dailymotion provides API to control a dailymotion webpage
// through emulating user actions. (ex: clicking)
package dailymotion

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mtbf/dom"
	"chromiumos/tast/testing"
)

// VideoPlayer represents the main <video> node in dailymotion page.
const VideoPlayer = "#dmp_Video"

// PlayVideo trigger VideoPlayer.play().
func PlayVideo(ctx context.Context, conn *chrome.Conn) error {
	return dom.PlayElement(ctx, conn, VideoPlayer)
}

// PauseVideo trigger VideoPlayer.pause().
func PauseVideo(ctx context.Context, conn *chrome.Conn) error {
	return dom.PauseElement(ctx, conn, VideoPlayer)
}

// ToggleFullScreen simulate keyboard input "F".
func ToggleFullScreen(ctx context.Context, conn *chrome.Conn) error {
	dom.WaitForElementBeingVisible(ctx, conn, VideoPlayer)
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

// OpenVideoSettings by select setting button and click it.
func OpenVideoSettings(ctx context.Context, conn *chrome.Conn) (err error) {
	// Dailymotion video need to mouse over or mouse click the video for showing settings button.
	mouse, err := input.Mouse(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot initialize mouse")
	}
	defer mouse.Close()

	// Ensure mouse position does not extend beyond the screen.
	mouse.Move(-10, 0)
	testing.Sleep(ctx, 1*time.Second)
	mouse.Move(10, 0)

	// Click settings button.
	const menuButton = ".np_ButtonSettings"
	if err = conn.Exec(ctx, dom.Query(menuButton)+".click()"); err != nil {
		mouse.Click()
	}
	return dom.WaitAndClick(ctx, conn, menuButton)
}

// Quality values are regex string for matching quality (<select>) options.
var Quality = map[string]string{
	"1080p": "1080",
	"720p":  "720",
	"480p":  "480",
	"380p":  "380",
	"240p":  "240",
	"144p":  "144",
}

// ChangeQuality changes the quality options.
func ChangeQuality(ctx context.Context, conn *chrome.Conn, quality string) (err error) {
	if err = OpenVideoSettings(ctx, conn); err != nil {
		return
	}

	const qualityButton = "#np_quality-menu-item"
	if err = dom.WaitAndClick(ctx, conn, qualityButton); err != nil {
		return
	}

	// Wait for animation.
	if err = testing.Sleep(ctx, 2*time.Second); err != nil {
		return
	}

	var qualityMenuItem = "#np_quality-menu-item--" + quality
	if err = dom.WaitAndClick(ctx, conn, qualityMenuItem); err != nil {
		return
	}

	const closeButton = "div.np_dialog-header.np_menu-header > button"
	if err = dom.WaitAndClick(ctx, conn, closeButton); err != nil {
		return
	}
	return nil
}

// GetCurrentTime return VideoPlayer.currentTime.
func GetCurrentTime(ctx context.Context, conn *chrome.Conn) (time float64, err error) {
	time, err = dom.GetElementCurrentTime(ctx, conn, VideoPlayer)
	return
}
