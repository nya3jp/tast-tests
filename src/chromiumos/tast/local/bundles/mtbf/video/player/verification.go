// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package player

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func parseDuration(t string) (float64, error) {
	f := ""
	ts := strings.Split(t, ":")
	switch len(ts) {
	case 1:
		f = fmt.Sprintf("%ss", ts[0])
	case 2:
		f = fmt.Sprintf("%sm%ss", ts[0], ts[1])
	case 3:
		f = fmt.Sprintf("%sh%sm%ss", ts[0], ts[1], ts[2])
	}

	d, err := time.ParseDuration(f)
	if err != nil {
		return -1, err
	}

	return d.Seconds(), nil
}

// GetVideoPlayingTime returns audio player playing time.
func GetVideoPlayingTime(ctx context.Context, conn *chrome.Conn) (playtime string, err error) {
	const javascript = `new Promise((resolve, reject) => {
		let playTime;
		const recursive = root => {
			if (root.children && root.children.length > 0) {
				root.children.forEach(child => recursive(child));
			} else if (root.role === 'inlineTextBox' && /\d*:\d*/.test(root.name)) {
				playTime = root.name;
			}
		};
		chrome.automation.getDesktop(root => recursive(root));
		if (playTime) {
			resolve(playTime);
		} else {
			reject("none time has been found.");
		}
	})`

	if err = conn.EvalPromise(ctx, javascript, &playtime); err != nil {
		//return -1, mtbferrors.New(mtbferrors.AudioPlayTime, err)
		return
	}
	return
}

// VerifyVideoPlaying verify video is still playing.
func VerifyVideoPlaying(ctx context.Context, conn *chrome.Conn, timeout time.Duration) (err error) {
	var currentTimeStr, previousTimeStr string
	if previousTimeStr, err = GetVideoPlayingTime(ctx, conn); err != nil {
		return
	}
	if err = testing.Sleep(ctx, timeout); err != nil {
		return mtbferrors.New(mtbferrors.ChromeSleep, err)
	}
	if currentTimeStr, err = GetVideoPlayingTime(ctx, conn); err != nil {
		return
	}

	currentTime, err := parseDuration(currentTimeStr)
	if err != nil {
		return mtbferrors.New(mtbferrors.VideoParseTime, nil, currentTimeStr)
	}

	previousTime, err := parseDuration(previousTimeStr)
	if err != nil {
		return mtbferrors.New(mtbferrors.VideoParseTime, nil, previousTimeStr)
	}

	if previousTime >= currentTime {
		return mtbferrors.New(mtbferrors.VideoPlay, nil, currentTime, previousTime, timeout.Seconds())
	}

	return nil
}

// VerifyVideoPausing verify audio is now pausing.
func VerifyVideoPausing(ctx context.Context, conn *chrome.Conn, timeout time.Duration) (err error) {
	var currentTimeStr, previousTimeStr string
	if previousTimeStr, err = GetVideoPlayingTime(ctx, conn); err != nil {
		return
	}
	if err = testing.Sleep(ctx, timeout); err != nil {
		return mtbferrors.New(mtbferrors.ChromeSleep, err)
	}
	if currentTimeStr, err = GetVideoPlayingTime(ctx, conn); err != nil {
		return
	}

	currentTime, err := parseDuration(currentTimeStr)
	if err != nil {
		return mtbferrors.New(mtbferrors.VideoParseTime, nil, currentTimeStr)
	}

	previousTime, err := parseDuration(previousTimeStr)
	if err != nil {
		return mtbferrors.New(mtbferrors.VideoParseTime, nil, previousTimeStr)
	}

	if currentTime > previousTime {
		return mtbferrors.New(mtbferrors.VideoPause, nil, currentTime, previousTime, timeout.Seconds())
	}

	return nil
}
