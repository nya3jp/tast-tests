// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"cienet.com/cats/node/sdk"
	"cienet.com/cats/node/sdk/ui"
	"cienet.com/cats/node/service"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/mtbf/multimedia"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF004NBCVideosPlaying,
		Desc:         "ARC++ Test NBC news app videos playing",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"meta.requestURL"},
		SoftwareDeps: []string{"chrome", "arc"},
		ServiceDeps: []string{
			"tast.mtbf.multimedia.VolumeService",
			"tast.mtbf.svc.CommService",
		},
	})
}

func MTBF004NBCVideosPlaying(ctx context.Context, s *testing.State) {
	desc := sdk.CaseDescription{
		Name:        "meta.MTBF004NBCVideosPlaying",
		Description: "ARC++ Test NBC news app videos playing",
		Timeout:     5 * time.Minute,
	}

	c, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "mtbf")
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.GRPCDialFail, err))
	}
	defer c.Close(ctx)

	volume := multimedia.NewVolumeServiceClient(c.Conn)
	defer volume.Set(ctx, &multimedia.VolumeRequest{Value: 50}) // set to default

	common.NodeDetachModeRunCase(ctx, s, desc, nbcVerifyVideosPlaying(volume), nbcExit)
}

type nbcPlayer struct {
	sdk.DelegateClient
	id      string
	center  string // center coordinate of the player
	btnFull string // center coordinate of button Fullscreen
	seekbar ui.UIElementSearchDesc
}

// ToggleVideoController toggles the video controller by clicking the
// video container.
func (p nbcPlayer) ToggleVideoController(ctx context.Context) {
	p.UIAClick(p.id).Coordinate(p.center).Snapshot(false).Do(ctx)
}

// PauseResume toggles the video playing status by clicking pause button.
func (p nbcPlayer) PauseResume(ctx context.Context) {
	p.UIAClick(p.id).Coordinate(p.center).Snapshot(false).Times(2).Intervals(300).Do(ctx, service.Sleep(0))
}

// CurrentTime returns the current time the video is playing.
func (p nbcPlayer) CurrentTime(ctx context.Context) string {
	const selector = "ID=com.zumobi.msnbc:id/time_current"

	p.PauseResume(ctx)
	defer p.PauseResume(ctx)

	elm, _ := p.UIASearchElement(p.id, selector).FailOnNotMatch(true).Do(ctx, service.Sleep(3*time.Second))
	testing.ContextLog(ctx, "current playing time ", elm.Text)
	return elm.Text
}

// IsPlaying returns if the video is playing.
func (p nbcPlayer) IsPlaying(ctx context.Context) bool {
	testing.ContextLog(ctx, "check if the video is playing")
	testing.Sleep(ctx, 3*time.Second) // wait for the controller to disappear
	defer testing.Sleep(ctx, 3*time.Second)

	t := p.CurrentTime(ctx)
	testing.Sleep(ctx, 3*time.Second)
	return t != p.CurrentTime(ctx)
}

// SelectSeekbar clicks the seekbar at random position.
func (p nbcPlayer) SelectSeekbar(ctx context.Context) {
	var (
		width      = (p.seekbar.BoundRight - p.seekbar.BoundLeft) * 3 / 4 // reserve last 1/4 for video playing
		rnd        = rand.New(rand.NewSource(time.Now().UnixNano()))
		coordinate = fmt.Sprintf("%d,%d", p.seekbar.BoundLeft+rnd.Int31n(width), p.seekbar.CenterY)
	)
	p.ToggleVideoController(ctx)
	p.UIAClick(p.id).Coordinate(coordinate).Do(ctx)
}

// Fullscreen makes the video displayed in fullscreen mode.
func (p nbcPlayer) Fullscreen(ctx context.Context) {
	p.ToggleVideoController(ctx)
	p.UIAClick(p.id).Coordinate(p.btnFull).Do(ctx, service.Sleep(3*time.Second))
}

func nbcNewPlayer(ctx context.Context, id string, dut sdk.DelegateClient) nbcPlayer {
	const (
		selectPlayer  = "ID=com.zumobi.msnbc:id/video_container::class=android.widget.FrameLayout"
		selectFull    = "ID=com.zumobi.msnbc:id/fullscreen"
		selectSeekbar = "ID=com.zumobi.msnbc:id/mediacontroller_progress::class=android.widget.SeekBar"
	)

	video, _ := dut.UIASearchElement(id, selectPlayer).FailOnNotMatch(true).Do(ctx)
	player := nbcPlayer{
		DelegateClient: dut,
		id:             id,
		center:         fmt.Sprintf("%d,%d", video.CenterX, video.CenterY),
	}
	player.PauseResume(ctx)
	defer player.PauseResume(ctx)

	full, _ := dut.UIASearchElement(id, selectFull).FailOnNotMatch(true).Snapshot(false).Do(ctx, service.Sleep(0))
	player.seekbar, _ = dut.UIASearchElement(id, selectSeekbar).FailOnNotMatch(true).Snapshot(false).Do(ctx, service.Sleep(3*time.Second))
	player.btnFull = fmt.Sprintf("%d,%d", full.CenterX, full.CenterY)
	return player
}

// nbcVerifyVideosPlaying holds the main process of the test case.
func nbcVerifyVideosPlaying(volume multimedia.VolumeServiceClient) sdk.Handler {
	return func(ctx context.Context, dut sdk.DelegateClient) (interface{}, error) {
		var id = ctx.Value(common.DutID).(string)
		const (
			act = "com.nbc.activities.MainActivity"
			pkg = "com.zumobi.msnbc"

			selectWatch   = "ID=com.zumobi.msnbc:id/bottom_nav_watch"
			selectWeekend = "text=MSNBC WEEKEND::ID=com.zumobi.msnbc:id/itemview_text"
			selectLists   = "ID=com.zumobi.msnbc:id/video_lists"
			selectDialog  = "ID=com.zumobi.msnbc:id/title::class=android.widget.TextView"
			selectToday   = "text=TODAY::ID=com.zumobi.msnbc:id/itemview_text"
			selectRoot    = "ID=android:id/content::class=android.widget.FrameLayout"
			selectPlayer  = "ID=com.zumobi.msnbc:id/video_container::class=android.widget.FrameLayout"
		)

		testing.ContextLog(ctx, "open NBC News app")
		dut.StartMainActivity(id, act, pkg).Do(ctx, service.Sleep(5*time.Second))

		testing.ContextLog(ctx, "navigate to WATCH page")
		dut.UIAClick(id).Selector(selectWatch).Do(ctx, service.Sleep(3*time.Second))

		testing.ContextLog(ctx, "play the latest video in section MSNBC WEEKEND")
		dut.UIAObjEventWait(id, selectWeekend, 10000, ui.ObjEventTypeAppear).Snapshot(false).Do(ctx)
		dut.UIAClick(id).Selector(selectWeekend).Do(ctx, service.Sleep(time.Second))

		elm, _ := dut.UIASearchElement(id, selectLists).FailOnNotMatch(true).Do(ctx)
		latest := fmt.Sprintf("%d,%d", elm.CenterX, elm.BoundTop+50)
		dut.UIAClick(id).Coordinate(latest).Do(ctx, service.Sleep(5*time.Second))

		// the layout is buggy when entering to the video page first time
		// so we get back to the playlists and click a video again
		testing.ContextLog(ctx, "return to WATCH page")
		dut.Press(id, ui.OprKeyEventCANCEL).Times(1).Do(ctx, service.Sleep(3*time.Second))

		// if a dialog shows up, press CANCEL
		if elm, _ = dut.UIASearchElement(id, selectDialog).Do(ctx); elm.Found {
			testing.ContextLog(ctx, "exit dialog")
			dut.Press(id, ui.OprKeyEventCANCEL).Times(1).Do(ctx)
		}

		testing.ContextLog(ctx, "play the latest video in section TODAY")
		dut.UIAClick(id).Selector(selectToday).Do(ctx, service.Sleep(time.Second))

		// after switching to "TODAY", the latest video of the section should located at
		// the same position, thus, we don't need to get the coordinate again
		dut.UIAClick(id).Coordinate(latest).Do(ctx, service.Sleep(3*time.Second))

		player := nbcNewPlayer(ctx, id, dut)
		testing.Sleep(ctx, 3*time.Second)
		if !player.IsPlaying(ctx) {
			return nil, mtbferrors.New(mtbferrors.GoogleNewsVideoNotPlay, nil)
		}

		testing.ContextLog(ctx, "verify fullscreen functionality")
		player.Fullscreen(ctx)

		// check if the video size is as same as the app's
		root, _ := dut.UIASearchElement(id, selectRoot).FailOnNotMatch(true).Snapshot(false).Do(ctx)
		video, _ := dut.UIASearchElement(id, selectPlayer).FailOnNotMatch(true).Do(ctx)
		if video.BoundLeft != root.BoundLeft || video.BoundRight != root.BoundRight ||
			video.BoundTop != root.BoundTop || video.BoundBottom != root.BoundBottom {
			return nil, mtbferrors.New(mtbferrors.VideoEnterFullSc, nil)
		}

		testing.ContextLog(ctx, "exit fullscreen mode")
		dut.Press(id, ui.OprKeyEventCANCEL).Times(1).Do(ctx)

		for i := 0; i < 2; i++ {
			testing.ContextLog(ctx, "play video at random seek position")
			player.SelectSeekbar(ctx)
			if !player.IsPlaying(ctx) {
				return nil, mtbferrors.New(mtbferrors.GoogleNewsVideoNotPlay, nil)
			}
		}

		testing.ContextLog(ctx, "verify volume functionality")
		for _, v := range []int64{10, 100} {
			if _, err := volume.Set(ctx, &multimedia.VolumeRequest{Value: v, Check: true}); err != nil {
				return nil, err
			}
			testing.Sleep(ctx, 2*time.Second)
		}

		return nil, nil
	}
}

func nbcExit(ctx context.Context, dut sdk.DelegateClient) (interface{}, error) {
	id := ctx.Value(common.DutID).(string)
	dut.Press(id, ui.OprKeyEventCANCEL).Times(3).Do(ctx)
	return nil, nil
}
