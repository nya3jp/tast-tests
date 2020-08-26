// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"os/exec"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/mtbf/camera"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MTBF054ModesFallback,
		Desc:     "Switch to portrait mode. Verifies after switch camera, mode selector will auto fallback to photo mode",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Data:     []string{"cca_ui.js"},
		ServiceDeps: []string{
			"tast.mtbf.svc.CommService",
			"tast.mtbf.camera.CameraService",
		},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"usbc.UsbcServer", "usbc.UsbDevID"},
		Timeout:      5 * time.Minute,
	})
}

// MTBF054ModesFallback test on portrait mode capable device (e.g. nocturne)
//  1. Switch to portrait mode
//  2. Plugin usb external camera
//  3. Switch to external camera while staying in portrait mode
//  4. The portrait mode icon should disappear and mode selector will auto fallback to photo mode.
func MTBF054ModesFallback(ctx context.Context, s *testing.State) {
	usbDevID := s.RequiredVar("usbc.UsbDevID")
	usbcServer := s.RequiredVar("usbc.UsbcServer")
	s.Log("Start to disable USB")
	if mtbferr := switchUSBRelay(s, usbcServer, usbDevID, close); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	// push cca_ui.js to daownload path
	common.DataFilesPrepare(ctx, s, []string{"cca_ui.js"})

	localOutDir := s.RPCHint().LocalOutDir

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "mtbf")
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.GRPCDialFail, err))
	}
	defer cl.Close(ctx)

	app := camera.NewCameraServiceClient(cl.Conn)
	defer app.CloseCamera(ctx, &camera.CloseCameraRequest{OutDir: localOutDir})

	s.Log("Start to switch to portrait mode")
	if _, mtbferr := app.SwitchToPortraitMode(ctx, &camera.SwitchToPortraitModeRequest{OutDir: localOutDir}); mtbferr != nil {
		common.Fatal(ctx, s, mtbferr)
	}

	s.Log("Start to enable USB")
	if mtbferr := switchUSBRelay(s, usbcServer, usbDevID, open); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		getNumOfCamerasRes, mtbferr := app.GetNumOfCameras(ctx, &camera.GetNumOfCamerasRequest{OutDir: localOutDir})
		if mtbferr != nil {
			return testing.PollBreak(mtbferr)
		}
		numCameras := getNumOfCamerasRes.Num
		s.Log("Get num of cameras = ", numCameras)
		if numCameras > 1 {
			s.Log("Switch camera")
			if _, err := app.SwitchCamera(ctx, &camera.SwitchCameraRequest{OutDir: localOutDir}); err != nil {
				return testing.PollBreak(mtbferrors.New(mtbferrors.CmrSwitch, err))
			}
			return nil
		} else if numCameras == 1 {
			return mtbferrors.New(mtbferrors.CmrCameraNum, nil)
		} else {
			return mtbferrors.New(mtbferrors.CmrNotFound, nil)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second}); err != nil {
		common.Fatal(ctx, s, err)
	}

	s.Log("Get mode state")
	getModeStateRes, err := app.GetModeState(ctx, &camera.GetModeStateRequest{OutDir: localOutDir, Mode: "photo"})
	if err != nil {
		common.Fatal(ctx, s, mtbferrors.New(mtbferrors.CmrAppState, err))
	} else if !getModeStateRes.Active {
		common.Fatal(ctx, s, mtbferrors.New(mtbferrors.CmrFallBack, nil))
	}

	s.Log("Check the portrait mode icon should disappear")
	portraitModeSelector := "Tast.isVisible('#modes-group > .mode-item:last-child')"
	checkElementExistReq := &camera.CheckElementExistRequest{
		OutDir:   localOutDir,
		Selector: portraitModeSelector,
		Expected: false,
	}
	if _, err := app.CheckElementExist(ctx, checkElementExistReq); err != nil {
		common.Fatal(ctx, s, mtbferrors.New(mtbferrors.CmrPortraitBtn, err))
	}
}

const (
	open  = "1"
	close = "0"
)

// switchUSBRelay enable relay with 1 and close with 0
func switchUSBRelay(s *testing.State, usbcServer, ID, status string) error {
	command := "sudo usbrelay " + ID + "=" + status
	s.Log("ssh ", usbcServer, " ", command)
	_, err := exec.Command("ssh", usbcServer, command).Output()
	if err != nil {
		return mtbferrors.New(mtbferrors.SwitchUSBRelay, err)
	}
	return nil
}
