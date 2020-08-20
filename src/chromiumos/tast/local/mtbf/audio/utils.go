// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"fmt"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
)

// GetAudioPlayingTime returns audio player playing time.
func GetAudioPlayingTime(ctx context.Context, conn *chrome.TestConn) (playtime int, err error) {
	javascript := fmt.Sprintf(
		`new Promise((resolve, reject) => {
			let playTime;
			%v
			const recursive = root => {
				const regex = /(\d*):(\d*) \/ \d*:\d*/;
				const target = { attributes: { role: 'staticText', name: regex } };
				const playTimeNodes = root.findAll(target);
				if (playTimeNodes.length) {
					const node = playTimeNodes[1];
					const [full, min, sec] = regex.exec(node.name);
					playTime = Number(min) * 60 + Number(sec);
					console.log('GetAudioPlayingTime: ', node.name, ', playtime: ', playTime);
				}
			};
			chrome.automation.getDesktop(root => recursive(getAudioPlayer(root)));
			if (Number.isInteger(playTime)) {
				resolve(playTime);
			} else {
				reject("none time has been found.");
			}
		})`, ScriptTemplate["getAudioPlayer"])

	if err = conn.EvalPromise(ctx, javascript, &playtime); err != nil {
		return -1, mtbferrors.New(mtbferrors.AudioPlayTime, err)
	}
	return
}

// GetOSVolume return so volume value of active audio output device.
func GetOSVolume(ctx context.Context, conn *chrome.TestConn) (volume int, err error) {
	javascript := `new Promise((resolve, reject) => {
		chrome.audio.getDevices({ streamTypes: ['OUTPUT'], isActive: true }, devices => { resolve(devices[0].level) });
	});`
	if err = conn.EvalPromise(ctx, javascript, &volume); err != nil {
		return volume, mtbferrors.New(mtbferrors.AudioGetOSVol, err)
	}
	return
}

// SetOSVolume sets operation system active sound device level.
func SetOSVolume(ctx context.Context, conn *chrome.TestConn, volume int) (err error) {
	javascrpt := fmt.Sprintf(`new Promise((resolve, reject) => {
		const adjustVolume = level => {
			chrome.audio.getDevices({ streamTypes: ['OUTPUT'], isActive: true }, devices => { chrome.audio.setProperties(devices[0].id, { level }, () => { }) });
		};
		adjustVolume(%d);
		resolve();
	});`, volume)
	if err = conn.EvalPromise(ctx, javascrpt, nil); err != nil {
		return mtbferrors.New(mtbferrors.AudioSetOSVol, err)
	}
	return
}

// IsOSVolumeMute return a bool indicate active system audio is mute or not.
func IsOSVolumeMute(ctx context.Context, conn *chrome.TestConn) (isMute bool, err error) {
	javascript := `new Promise((resolve, reject) => {
		chrome.audio.getMute('OUTPUT', isMute => { resolve(isMute) });
	});`
	if err = conn.EvalPromise(ctx, javascript, &isMute); err != nil {
		return isMute, mtbferrors.New(mtbferrors.AudioGetOSMute, err)
	}
	return
}

// SetOSVolumeMute set operation system active sound device mute or not.
func SetOSVolumeMute(ctx context.Context, conn *chrome.TestConn, isMute bool) (err error) {
	javascript := fmt.Sprintf(`chrome.audio.setMute('OUTPUT', %t);`, isMute)
	if err = conn.Eval(ctx, javascript, nil); err != nil {
		return mtbferrors.New(mtbferrors.AudioSetOSMute, err)
	}
	return nil
}
