// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dom

import (
	"context"
	"fmt"

	"chromiumos/tast/local/chrome"
)

// WaitForDocumentReady waits for the ready state of the document
func WaitForDocumentReady(ctx context.Context, conn *chrome.Conn) error {
	return conn.WaitForExpr(ctx, "document.readyState === 'complete'")
}

// WaitForElementBeingVisible waits for an element to be visible
func WaitForElementBeingVisible(ctx context.Context, conn *chrome.Conn, selector string) error {
	return conn.WaitForExprFailOnErr(ctx, Query(selector))
}

// ClickElement clicks an element
func ClickElement(ctx context.Context, conn *chrome.Conn, selector string) error {
	return conn.Exec(ctx, Click(selector))
}

// FocusElement puts focus on an element
func FocusElement(ctx context.Context, conn *chrome.Conn, selector string) error {
	return conn.Exec(ctx, Focus(selector))
}

// RightClickElement triggers contextmenu event on selected dom
func RightClickElement(ctx context.Context, conn *chrome.Conn, selector string) error {
	return conn.Exec(ctx, DispatchEvent(selector, "new CustomEvent('contextmenu')"))
}

// WaitAndClick waits and clicks target element by given selector
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

// PlayElementWithTimeout plays the video with timeout value
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
	return
}

// FastJumpElement does a fast jump to specific time from current time.
func FastJumpElement(ctx context.Context, conn *chrome.Conn, selector string, jumpTime float64) error {
	return conn.Exec(ctx, fmt.Sprintf("%s.currentTime += %f", Query(selector), jumpTime))
}

// FastForwardTime is the time media element fast forward in seconds.
const FastForwardTime = 10

// FastForwardElement does a fast forward on a media element by 10 secs.
func FastForwardElement(ctx context.Context, conn *chrome.Conn, selector string) error {
	return FastJumpElement(ctx, conn, selector, FastForwardTime)
}

// FastRewindTime is the time media element fast rewind in seconds.
const FastRewindTime = -10

// FastRewindElement does a fast rewind on a media element by 10 secs.
func FastRewindElement(ctx context.Context, conn *chrome.Conn, selector string) error {
	return FastJumpElement(ctx, conn, selector, FastRewindTime)
}

// ReloadPage calls window.location.reload() to refresh page.
func ReloadPage(ctx context.Context, conn *chrome.Conn) error {
	return conn.Exec(ctx, "window.location.reload()")
}
