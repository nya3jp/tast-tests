// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/bluetooth/btconn"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mtbf/audio"
	mtbfFilesapp "chromiumos/tast/local/mtbf/ui/filesapp"
	"chromiumos/tast/local/ui/filesapp"
	"chromiumos/tast/testing"
)

const (
	// hangoutURL = "https://hangouts.google.com/call/vGZceZRfWBT7ezps5mlUAEEM"
	joinBtn   = `document.querySelector("#yDmH0d > div.WOi1Wb > div.GhN39b > div > div > div > div > div > span")`
	audioFile = "audio.mp3"
	//musicURL = "http://storage.googleapis.com/chromiumos-test-assets-public/AV-testing-files/02%20Daaru%20Desi.m4a"
	//joinBtnClick = `document.querySelector("#yDmH0d > div.WOi1Wb > div.GhN39b > div > div > div.ECPdDc > div.FtvmQ > div > div.ZFr60d.CeoRYc").click()`
	//joinBtn = `document.querySelector("#yDmH0d > div.WOi1Wb > div.GhN39b > div > div > div.ECPdDc > div.F1FBvf > div > span")`
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF039MusicHangout,
		Desc:         "MTBF039MusicHangout is a sub case of MTBF039 to join play music and a hangouts call",
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoginReuse(),
		Attr:         []string{"group:mainline"},
		Contacts:     []string{"xliu@cienet.com"},
		Vars:         []string{"bt.a2dp.deviceName", "video.hangoutsURL"},
		Data:         []string{audioFile},
	})
}

// MTBF039MusicHangout is a sub case of MTBF039 to join play music and a hangouts call
func MTBF039MusicHangout(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	btDeviceName, ok := s.Var("bt.a2dp.deviceName")
	if !ok {
		s.Fatal(mtbferrors.New(mtbferrors.OSVarRead, nil, "bt.a2dp.deviceName"))
	}

	hangoutURL, ok := s.Var("video.hangoutsURL")
	if !ok {
		s.Fatal(mtbferrors.New(mtbferrors.OSVarRead, nil, "video.hangoutsURL"))
	}

	s.Logf("MTBF039MusicHangout - btDeviceName: %s, hangoutURL: %s", btDeviceName, hangoutURL)

	btConn, err := btconn.New(ctx, s, cr, nil)
	if err != nil {
		s.Fatal("MTBF failed: ", err)
	}
	defer btConn.Close()

	s.Log("btDeviceName: ", btDeviceName)
	btAddress, err := btConn.GetAddress(btDeviceName)
	if err != nil {
		s.Fatal("MTBF failed: ", err)
	}

	s.Log("btAddress: ", btAddress)
	btConsole, err := btconn.NewBtConsole(ctx, s)
	if err != nil {
		s.Fatal("MTBF failed: ", err)
	}
	defer btConsole.Close()

	if err := btConsole.Connect(btAddress); err != nil {
		s.Fatal("MTBF failed: ", err)
	}

	_, err = btConsole.CheckScanning(true)
	if err != nil {
		s.Fatal("MTBF failed: ", err)
	}

	if err := playMusic(ctx, s, cr); err != nil {
		s.Fatal("MTBF failed: ", err)
	}

	isA2dp, err := btConsole.IsA2dp(btAddress)
	if err != nil {
		s.Fatal("MTBF failed: ", err)
	}

	s.Log("isA2dp: ", isA2dp)
	if !isA2dp {
		s.Fatal(mtbferrors.New(mtbferrors.BTNotA2DP, nil, btDeviceName))
	}

	if err := joinHangout(ctx, s, hangoutURL); err != nil {
		s.Fatal("MTBF failed: ", err)
	}

	isHsp, err := btConsole.IsHsp(btAddress)
	if err != nil {
		s.Fatal("MTBF failed: ", err)
	}

	s.Log("Joined hangout. isHsp: ", isHsp)
	if !isHsp {
		s.Fatal(mtbferrors.New(mtbferrors.BTNotHSP, nil, btDeviceName))
	}

	if connected, err := btConsole.IsConnected(btAddress); err != nil {
		s.Fatal("MTBF failed: ", err)
	} else if !connected {
		s.Fatal(mtbferrors.New(mtbferrors.BTConnectFailed, nil, btAddress))
	}

	//info, err := btConsole.GetDeviceInfo(btAddress)
	//if err != nil {
	//	s.Fatal("MTBF failed", err)
	//	//}
	//
	//if strings.Contains(info, "Connected: no") {
	//	s.Fatal(mtbferrors.New(mtbferrors.BTConnectFailed, nil, btAddress))
	//}

	s.Log("Finished")
}

func joinHangout(ctx context.Context, s *testing.State, hangoutURL string) error {
	cr := s.PreValue().(*chrome.Chrome)
	conn, err := cr.NewConn(ctx, hangoutURL)
	if err != nil {
		return mtbferrors.New(mtbferrors.ChoromeJoinHangout, err, hangoutURL)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	s.Log("joinBtn: ", joinBtn)
	testing.Sleep(ctx, 3*time.Second)

	if err := conn.WaitForExprWithTimeout(ctx, joinBtn, 30*time.Second); err != nil {
		return mtbferrors.New(mtbferrors.ChoromeJoinHangout, err, hangoutURL)
	}

	testing.Sleep(ctx, 3*time.Second)

	if err := conn.Exec(ctx, joinBtn+".click()"); err != nil {
		return mtbferrors.New(mtbferrors.ChoromeJoinHangout, err, hangoutURL)
	}

	testing.Sleep(ctx, 10*time.Second)
	return nil
}

func playMusic(ctx context.Context, s *testing.State, cr *chrome.Chrome) error {
	s.Log("Open the test API")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return mtbferrors.New(mtbferrors.ChromeTestConn, err)
	}
	defer tconn.Close()

	files, err := mtbfFilesapp.Launch(ctx, tconn)
	if err != nil {
		return err
	}
	defer filesapp.Close(ctx, tconn)

	if err := audio.PlayFromDownloadsFolder(ctx, files, s.DataPath(audioFile), audioFile); err != nil {
		return err
	}

	testing.Sleep(ctx, 5*time.Second)

	if err := audio.Close(ctx, tconn); err != nil {
		return err
	}

	return nil
}
