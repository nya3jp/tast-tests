// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gamingproxycuj

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/media/devtools"
	"chromiumos/tast/testing"
)

// VideoOption defines the name and resolution of the video option.
type VideoOption struct {
	name       string
	resolution string
}

// Options for video options.
var (
	H264DASH1080P30FPS = VideoOption{"H264 DASH 30 FPS", "1920x1080"}
	H264DASH1080P60FPS = VideoOption{"H264 DASH 60 FPS", "1920x1080"}
	H264DASH4K60FPS    = VideoOption{"H264 DASH 60 FPS", "3840x2160"}
	AV1DASH60FPS       = VideoOption{"AV1 DASH 60FPS", "3840x2026"}
)

// CrosVideo defines the struct related to cros video web.
type CrosVideo struct {
	ui   *uiauto.Context
	conn *chrome.Conn
}

var crosVideoWebArea = nodewith.NameContaining("CrosVideo Test").Role(role.RootWebArea)

// NewCrosVideo open cros video URL and return the cros video instance.
func NewCrosVideo(ctx context.Context, tconn *chrome.TestConn, uiHandler cuj.UIActionHandler, br *browser.Browser) (*CrosVideo, error) {
	conn, err := uiHandler.NewChromeTab(ctx, br, cuj.CrosVideoURL, true)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open cros video URL %q", cuj.CrosVideoURL)
	}
	return &CrosVideo{
		ui:   uiauto.New(tconn),
		conn: conn,
	}, nil
}

// Play plays the cros video with spectific video option and resolution.
func (v *CrosVideo) Play(option VideoOption) action.Action {
	const retryTimes = 3
	defaultCell := nodewith.Name(string(H264DASH1080P30FPS.name)).Role(role.Cell).Ancestor(crosVideoWebArea)
	expectedItem := nodewith.Name(string(option.name)).Role(role.ListBoxOption).Ancestor(crosVideoWebArea)
	expectedCell := nodewith.Name(string(option.name)).Role(role.Cell).Ancestor(crosVideoWebArea)
	selectTestManifest := uiauto.NamedCombine("select "+string(option.name),
		v.ui.LeftClick(defaultCell),
		v.ui.LeftClick(expectedItem),
		v.ui.WaitUntilExists(expectedCell))
	loopVideoCell := nodewith.Name("Loop video").Role(role.Cell).Ancestor(crosVideoWebArea)
	loopVideoCheckBox := nodewith.Role(role.CheckBox).Ancestor(loopVideoCell)
	checkLoopVideoEnabled := func(ctx context.Context) error {
		if node, err := v.ui.Info(ctx, loopVideoCheckBox); err != nil {
			return err
		} else if node.Checked == "false" {
			return errors.New("loop video has not been enabled")
		}
		return nil
	}
	loadStream := nodewith.Name("Load stream").Role(role.Button).Ancestor(crosVideoWebArea)
	selectResolution := func(ctx context.Context) error {
		expectedResolution := nodewith.NameContaining(option.resolution).Role(role.Cell).Ancestor(crosVideoWebArea)
		if err := v.ui.WaitUntilExists(expectedResolution)(ctx); err == nil {
			return nil
		}
		defaultResolution := nodewith.NameContaining("bits").Role(role.Cell).Ancestor(crosVideoWebArea)
		resolution := nodewith.NameContaining(option.resolution).Role(role.ListBoxOption).Ancestor(crosVideoWebArea)
		return uiauto.NamedCombine("select resolution "+option.resolution,
			v.ui.LeftClick(defaultResolution),
			v.ui.LeftClick(resolution),
			v.ui.WaitUntilExists(expectedResolution),
			v.VerifyPlaying,
		)(ctx)
	}

	return v.ui.Retry(retryTimes, uiauto.NamedCombine("play the video with "+string(option.name),
		uiauto.IfSuccessThen(v.ui.Gone(expectedCell), selectTestManifest),
		v.ui.DoDefaultUntil(loopVideoCheckBox, checkLoopVideoEnabled),
		v.ui.LeftClick(loadStream),
		v.VerifyPlaying,
		selectResolution,
		v.printVideoDecoderName,
	))
}

// Pause pauses the cros video.
func (v *CrosVideo) Pause() action.Action {
	video := nodewith.Role(role.Video).Ancestor(crosVideoWebArea)
	return uiauto.NamedAction("pause the video", v.ui.LeftClick(video))
}

// VerifyPlaying the cros video is playing.
func (v *CrosVideo) VerifyPlaying(ctx context.Context) error {
	testing.ContextLog(ctx, "Verify the cros video is playing")
	return testing.Poll(ctx, func(ctx context.Context) error {
		var isPaused bool
		if err := v.conn.Call(ctx, &isPaused, `() => document.querySelector("#video").paused`); err != nil {
			return errors.Wrap(err, "failed to get playing state")
		}
		if isPaused {
			return errors.New("the cros video is not playing")
		}
		testing.ContextLog(ctx, "The cros video is playing")
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// Exists returns whether the video exists.
func (v *CrosVideo) Exists() action.Action {
	video := nodewith.Role(role.Video).Ancestor(crosVideoWebArea)
	return v.ui.Exists(video)
}

// FramesData returns frames data including decoded frames, dropped frames and dropped frames percent.
func (v *CrosVideo) FramesData(ctx context.Context) (decodedFrames, droppedFrames, droppedFramesPer float64, err error) {
	if err := v.conn.Call(ctx, &decodedFrames, `() => parseFloat(document.querySelector("#decodedFramesDebug").textContent)`); err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to get decoded frames")
	}
	if err := v.conn.Call(ctx, &droppedFrames, `() => parseFloat(document.querySelector("#droppedFramesDebug").textContent)`); err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to get dropped frames")
	}
	if err := v.conn.Call(ctx, &droppedFramesPer, `() => parseFloat(document.querySelector("#droppedFramesPerDebug").textContent)`); err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to get dropped frames percent")
	}
	testing.ContextLogf(ctx, "Get frames data: decoded frames=%f, dropped frames=%f, dropped frames percent=%f", decodedFrames, droppedFrames, droppedFramesPer)
	return decodedFrames, droppedFrames, droppedFramesPer, nil
}

// Close the cros video page.
func (v *CrosVideo) Close(ctx context.Context) {
	v.conn.CloseTarget(ctx)
	v.conn.Close()
}

// printVideoDecoderName prints the video decoder name.
func (v *CrosVideo) printVideoDecoderName(ctx context.Context) error {
	observer, err := v.conn.GetMediaPropertiesChangedObserver(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to retrieve DevTools Media messages")
	}
	_, decoderName, err := devtools.GetVideoDecoder(ctx, observer, cuj.CrosVideoURL)
	if err != nil {
		return errors.Wrap(err, "failed to parse Media DevTools")
	}
	testing.ContextLog(ctx, "Video decoder name: ", decoderName)
	return nil
}
