// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// IsPlaying verify audio is still playing.
func IsPlaying(ctx context.Context, conn *chrome.TestConn, timeout time.Duration) (err error) {
	var currentTime, previousTime int
	if previousTime, err = GetAudioPlayingTime(ctx, conn); err != nil {
		return
	}
	if err = testing.Sleep(ctx, timeout); err != nil {
		return mtbferrors.New(mtbferrors.ChromeSleep, err)
	}
	if currentTime, err = GetAudioPlayingTime(ctx, conn); err != nil {
		return
	}
	if previousTime == currentTime {
		return mtbferrors.New(mtbferrors.AudioPlayFwd, nil, currentTime, previousTime, timeout.Seconds())
	}

	return nil
}

// IsPausing verify audio is now pausing.
func IsPausing(ctx context.Context, conn *chrome.TestConn, timeout time.Duration) (err error) {
	var currentTime, previousTime int
	if previousTime, err = GetAudioPlayingTime(ctx, conn); err != nil {
		return
	}
	if err = testing.Sleep(ctx, timeout); err != nil {
		return mtbferrors.New(mtbferrors.ChromeSleep, err)
	}
	if currentTime, err = GetAudioPlayingTime(ctx, conn); err != nil {
		return
	}
	if currentTime > previousTime {
		return mtbferrors.New(mtbferrors.AudioPause, nil, timeout.Seconds(), currentTime, previousTime)
	}

	return nil
}

// CheckOSVolume checks whether the OS volume is same with given volume.
func CheckOSVolume(ctx context.Context, conn *chrome.TestConn, volume int) (err error) {
	if err = SetOSVolume(ctx, conn, volume); err != nil {
		return
	}

	systemVolume := 0
	systemVolume, err = GetOSVolume(ctx, conn)
	if err != nil {
		return
	}
	if systemVolume != volume {
		return mtbferrors.New(mtbferrors.AudioVolume, err, systemVolume, volume)
	}

	return nil
}
