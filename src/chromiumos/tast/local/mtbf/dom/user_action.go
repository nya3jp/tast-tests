// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dom

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// WaitForDocumentReady waits for the ready state of the document.
func WaitForDocumentReady(ctx context.Context, conn *chrome.Conn) error {
	return conn.WaitForExpr(ctx, "document.readyState === 'complete'")
}

// WaitForElementBeingVisible waits for an element to be visible.
func WaitForElementBeingVisible(ctx context.Context, conn *chrome.Conn, selector string) error {
	return conn.WaitForExprFailOnErr(ctx, Query(selector))
}

// ClickElement clicks an element.
func ClickElement(ctx context.Context, conn *chrome.Conn, selector string) error {
	return conn.Exec(ctx, Click(selector))
}

// FocusElement puts focus on an element.
func FocusElement(ctx context.Context, conn *chrome.Conn, selector string) error {
	return conn.Exec(ctx, Focus(selector))
}

// RightClickElement triggers contextmenu event on selected dom.
func RightClickElement(ctx context.Context, conn *chrome.Conn, selector string) error {
	return conn.Exec(ctx, DispatchEvent(selector, "new CustomEvent('contextmenu')"))
}

// WaitAndClick waits and clicks target element by given selector.
func WaitAndClick(ctx context.Context, conn *chrome.Conn, selector string) (err error) {
	if err = WaitForElementBeingVisible(ctx, conn, selector); err != nil {
		return
	}

	if err = ClickElement(ctx, conn, selector); err != nil {
		return
	}

	return nil
}

// IsElementExists queries selector and see if it exists.
func IsElementExists(ctx context.Context, conn *chrome.Conn, selector string) (isExists bool, err error) {
	err = conn.Eval(ctx, fmt.Sprintf("%s !== null;", Query(selector)), &isExists)
	return
}

// PlayElementWithTimeout plays the video with timeout value.
func PlayElementWithTimeout(ctx context.Context, conn *chrome.Conn, selector string, timeout int64) error {
	expr := fmt.Sprintf(`
		let timeout = new Promise((resolve, reject) => {
			let wait = setTimeout(() => {
				clearTimeout(wait);
				reject('play timeout');
			}, %d)
		})

		let play = %s;

		Promise.race([timeout, play])
	`, timeout, Query(selector)+".play()")

	return conn.EvalPromise(ctx, expr, nil)
}

// PlayElement by executing play() on media element.
func PlayElement(ctx context.Context, conn *chrome.Conn, selector string) error {
	return conn.EvalPromise(ctx, Query(selector)+".play()", nil)
}

// PauseElement by executing pause() on media element.
func PauseElement(ctx context.Context, conn *chrome.Conn, selector string) error {
	return conn.Exec(ctx, Query(selector)+".pause()")
}

// GetElementCurrentTime returns currentTime on media element.
func GetElementCurrentTime(ctx context.Context, conn *chrome.Conn, selector string) (time float64, err error) {
	err = conn.Eval(ctx, Query(selector)+".currentTime", &time)
	return time, err
}

// WaitForReadyState does wait video ready state then return.
func WaitForReadyState(ctx context.Context, conn *chrome.Conn, selector string, timeout, interval time.Duration) error {
	queryCode := fmt.Sprintf("new Promise((resolve, reject) => { let video = document.querySelector(%q); resolve(video.readyState === 4 && video.buffered.length > 0); });", selector)

	// Wait for element to appear.
	err := testing.Poll(ctx, func(ctx context.Context) error {
		var pageReady bool
		err := conn.EvalPromise(ctx, queryCode, &pageReady)
		if err != nil {
			return err
		}
		if pageReady {
			return nil
		}
		return err
	}, &testing.PollOptions{
		Timeout:  timeout,
		Interval: interval,
	})
	if err != nil {
		return err
	}
	return nil
}

// ElementReadyState returns media element ready state.
// HAVE_NOTHING 	0 	No information is available about the media resource.
// HAVE_METADATA 	1 	Enough of the media resource has been retrieved that the metadata attributes are initialized. Seeking will no longer raise an exception.
// HAVE_CURRENT_DATA 	2 	Data is available for the current playback position, but not enough to actually play more than one frame.
// HAVE_FUTURE_DATA 	3 	Data for the current playback position as well as for at least a little bit of time into the future is available (in other words, at least two frames of video, for example).
// HAVE_ENOUGH_DATA 	4 	Enough data is available and the download rate is high enough that the media can be played through to the end without interruption.
func ElementReadyState(ctx context.Context, conn *chrome.Conn, selector string) (readyState int, err error) {
	err = conn.Eval(ctx, Query(selector)+".readyState", &readyState)
	return readyState, err
}

// ElementNetworkState returns medial element network state.
// NETWORK_EMPTY 	0 	There is no data yet. Also, readyState is HAVE_NOTHING.
// NETWORK_IDLE 	1 	HTMLMediaElement is active and has selected a resource, but is not using the network.
// NETWORK_LOADING 	2 	The browser is downloading HTMLMediaElement data.
// NETWORK_NO_SOURCE 	3 	No HTMLMediaElement src found.
func ElementNetworkState(ctx context.Context, conn *chrome.Conn, selector string) (networkState int, err error) {
	err = conn.Eval(ctx, Query(selector)+".networkState", &networkState)
	return networkState, err
}

// FastJumpElement does a fast jump to specific time from current time.
func FastJumpElement(ctx context.Context, conn *chrome.Conn, selector string, jumpTime float64) error {
	return conn.Exec(ctx, fmt.Sprintf("%s.currentTime += %f", Query(selector), jumpTime))
}

// FastForwardTime is the time media element fast forward in seconds.
const FastForwardTime = 3

// FastForwardElement does a fast forward on a media element by 10 secs.
func FastForwardElement(ctx context.Context, conn *chrome.Conn, selector string) error {
	return FastJumpElement(ctx, conn, selector, FastForwardTime)
}

// FastRewindTime is the time media element fast rewind in seconds.
const FastRewindTime = -3

// FastRewindElement does a fast rewind on a media element by 10 secs.
func FastRewindElement(ctx context.Context, conn *chrome.Conn, selector string) error {
	return FastJumpElement(ctx, conn, selector, FastRewindTime)
}

// ReloadPage calls window.location.reload() to refresh page.
func ReloadPage(ctx context.Context, conn *chrome.Conn) error {
	return conn.Exec(ctx, "window.location.reload()")
}
