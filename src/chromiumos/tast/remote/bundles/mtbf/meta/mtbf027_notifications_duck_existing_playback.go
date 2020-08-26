// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"time"

	"cienet.com/cats/node/sdk"
	"cienet.com/cats/node/sdk/ui"
	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/remote/bundles/mtbf/meta/cats/utils"
	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/mtbf/multimedia"
	"chromiumos/tast/services/mtbf/svc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MTBF027NotificationsDuckExistingPlayback,
		Desc:     "Short playbacks/notifications should duck the existing playback",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars: []string{
			"meta.requestURL",
			"contact",
			"video.youtubeVideo",
			"video.shortDuckingAudioURL",
		},
		ServiceDeps: []string{
			"tast.mtbf.svc.CommService",
			"tast.mtbf.multimedia.YoutubeService",
			"tast.mtbf.svc.WebService",
		},
		SoftwareDeps: []string{"chrome", "arc"},
	})
}

// MTBF027NotificationsDuckExistingPlayback Run CATS case
// Procedure:
// 1. Start playing audio/video from browser or default player Ex: YouTube video.
// 2. Play any short (<5 seconds) ducking audio, ex: http://rebeccahughes.github.io/media/audio-focus/transient_duck.html
// 3. Observe behavior.
// 4. Let YouTube video play and open Gmail in another page.
// 5. Send chat message from different device to this Gmail account to get notification (make sure notification is enabled from Gmail).
// 6. Observe video behavior.
func MTBF027NotificationsDuckExistingPlayback(ctx context.Context, s *testing.State) {
	contact, ok := s.Var("contact")
	if !ok {
		s.Fatal(mtbferrors.New(mtbferrors.OSVarRead, nil, "contact"))
	}

	videoURL := common.Add1SecondForURL(s.RequiredVar("video.youtubeVideo"))
	shortDuckingAudioURL := s.RequiredVar("video.shortDuckingAudioURL")
	gmailURL := "https://mail.google.com/mail/u/0/#inbox"

	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF027NotificationsDuckExistingPlayback",
		Description: "Short playbacks/notifications should duck the existing playback",
	}

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		compDev := utils.NewDevice(client, common.CompanionID)

		cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "mtbf")
		if err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.GRPCDialFail, err))
		}
		defer cl.Close(ctx)

		web := svc.NewWebServiceClient(cl.Conn)
		youtube := multimedia.NewYoutubeServiceClient(cl.Conn)

		s.Log("Open youtube url")
		if _, mtbferr := web.OpenURL(ctx, &svc.OpenURLRequest{Url: videoURL}); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		s.Log("Open short ducking audio url")
		if _, mtbferr := web.OpenURL(ctx, &svc.OpenURLRequest{Url: shortDuckingAudioURL}); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		testing.Sleep(ctx, 3*time.Second)
		s.Log("Play short ducking audio")
		if _, mtbferr := web.PlayElement(ctx, &svc.PlayElementRequest{Url: shortDuckingAudioURL, Selector: "body > audio"}); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		testing.Sleep(ctx, 5*time.Second)
		s.Log("Close short ducking audio url")
		if _, mtbferr := web.CloseURL(ctx, &svc.CloseURLRequest{Url: shortDuckingAudioURL}); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		s.Log("Verify youtube is playing")
		if _, mtbferr := youtube.IsPlaying(ctx, &multimedia.IsPlayingRequest{
			Url:     videoURL,
			Timeout: 5,
		}); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		s.Log("Open Gmail url")
		if _, mtbferr := web.OpenURL(ctx, &svc.OpenURLRequest{Url: gmailURL}); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		s.Log("Send Hangouts message")
		if mtbferr := compDev.SendHangoutsMessage(ctx, contact); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		s.Log("Verify received message")
		if res, mtbferr := web.IsGmailChatRoomExists(ctx, &empty.Empty{}); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		} else if !res.IsExists {
			common.Fatal(ctx, s, mtbferrors.New(mtbferrors.AudioNoMsg, nil))
		}

		s.Log("Verify youtube is playing")
		if _, mtbferr := youtube.IsPlaying(ctx, &multimedia.IsPlayingRequest{
			Url:     videoURL,
			Timeout: 5,
		}); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}
		return nil, nil
	}

	cleanUp := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		compDev := utils.NewDevice(client, common.CompanionID)
		cleanup027DUTAndCompanionPhone(ctx, compDev)

		cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "mtbf")
		if err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.GRPCDialFail, err))
		}
		defer cl.Close(ctx)

		web := svc.NewWebServiceClient(cl.Conn)
		s.Log("Close youtube url")
		if _, mtbferr := web.CloseURL(ctx, &svc.CloseURLRequest{Url: videoURL}); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		s.Log("Close gmail url")
		if _, mtbferr := web.CloseURL(ctx, &svc.CloseURLRequest{Url: gmailURL}); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		return nil, nil
	}

	common.NodeDetachModeRunCase(ctx, s, caseDesc, testRun, cleanUp)
}

func cleanup027DUTAndCompanionPhone(ctx context.Context, dut *utils.Device) {
	dut.Client.Press(dut.DeviceID, ui.OprKeyEventCANCEL).Times(3).Do(ctx)
	dut.Client.Press(dut.DeviceID, ui.OprKeyEventHOME).Do(ctx)
}
