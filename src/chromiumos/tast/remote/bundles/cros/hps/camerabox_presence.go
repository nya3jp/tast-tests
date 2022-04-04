// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hps

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/camera/chart"
	"chromiumos/tast/common/hps/hpsutil"
	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CameraboxPresence,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that the HPS can correctly detect presence from another tablet",
		Data: []string{hpsutil.PersonPresentPageArchiveFilename,
			hpsutil.P2PowerCycleFilename},
		Contacts: []string{
			"eunicesun@google.com",
			"mblsha@google.com",
			"chromeos-hps-swe@google.com",
		},
		Attr:         []string{"group:camerabox", "group:hps"},
		Timeout:      20 * time.Minute,
		SoftwareDeps: []string{"hps", "chrome", caps.BuiltinCamera},
		Vars:         []string{"tablet"},
	})
}

func CameraboxPresence(ctx context.Context, s *testing.State) {
	// Connecting to DUT with HPS and send powercycle file to it
	d := s.DUT()
	cl, err := rpc.Dial(ctx, d, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the DUT: ", err)
	}
	defer cl.Close(ctx)

	// Power-cycle HPS after two people are already visible on the screen so that
	// the HPS would already be able to correctly adjust the exposure right from the start.

	// Sending powercycle python file to DUT with HPS

	archive := s.DataPath(hpsutil.PersonPresentPageArchiveFilename)
	powercycleDirPath, powercycleFilePath, err := CreateTmpDir(ctx, d.Conn())
	if err != nil {
		s.Fatal("Tmp dir creation failed on DUT")
	}
	filePaths, err := SendImageTar(ctx, d.Conn(), archive, powercycleDirPath, powercycleFilePath)

	// Creating hps context
	hctx, err := hpsutil.NewHpsContext(ctx, powercycleFilePath, hpsutil.DeviceTypeBuiltin, s.OutDir(), d.Conn())
	if err != nil {
		s.Fatal("Error creating HpsContext: ", err)
	}
	// Connecting to the other tablet that will render the picture
	var chartAddr string
	if altAddr, ok := s.Var("tablet"); ok {
		chartAddr = altAddr
	}

	c, hostPaths, err := chart.New(ctx, d, chartAddr, s.OutDir(), filePaths)
	if err != nil {
		s.Fatal("Put picture failed: ", err)
	}
	testing.ContextLog(ctx, "hostPaths: ", hostPaths)

	for i := 0; i < 2; i++ {
		// for no person
		c.Display(ctx, hostPaths[0])
		if err := numPersonDetect(hctx, strconv.Itoa(i)); err != nil {
			s.Fatal("Failed to run N presence ops: ", err)
		}

		// for one/two person present
		c.Display(ctx, hostPaths[i+1])
		if err := numPersonDetect(hctx, strconv.Itoa(i)); err != nil {
			s.Fatal("Failed to run N presence ops: ", err)
		}
	}
}

func numPersonDetect(hctx *hpsutil.HpsContext, feature string) error {

	if _, err := hpsutil.EnablePresence(hctx, feature); err != nil {
		return errors.Wrap(err, "enablePresence failed")
	}

	if _, err := hpsutil.WaitForNPresenceOps(hctx, hpsutil.WaitNOpsBeforeStart, feature); err != nil {
		return errors.Wrap(err, "failed to run N presence ops")
	}

	_, err := hpsutil.WaitForNPresenceOps(hctx, hpsutil.GetNOpsToVerifyPresenceWorks, feature)
	if err != nil {
		return errors.Wrap(err, "failed to run N presence ops")
	}
	return nil
}
