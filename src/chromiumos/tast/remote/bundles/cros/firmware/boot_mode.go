// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/rpc"
	fwpb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BootMode,
		Desc:         "Verifies that remote tests can boot the DUT into, and confirm that the DUT is in, the different firmware modes (normal, dev, and recovery)",
		Contacts:     []string{"cros-fw-engprod@google.com"},
		Data:         firmware.ConfigDatafiles(),
		ServiceDeps:  []string{"tast.cros.firmware.UtilsService"},
		SoftwareDeps: []string{"crossystem"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name: "normal",
			Val:  []fwCommon.BootMode{fwCommon.BootModeNormal, fwCommon.BootModeNormal},
		}, {
			Name: "rec",
			Val:  []fwCommon.BootMode{fwCommon.BootModeNormal, fwCommon.BootModeRecovery, fwCommon.BootModeNormal},
		}},
		Vars: []string{"servo"},
	})
}

func BootMode(ctx context.Context, s *testing.State) {
	modes := s.Param().([]fwCommon.BootMode)

	// Connect to the gRPC server on the DUT.
	d := s.DUT()
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC: ", err)
	}
	// The identity of cl will change as the RPC client reconnects after each reboot.
	// So, defer closing only the most up-to-date cl.
	defer func() {
		if cl != nil {
			cl.Close(ctx)
		}
	}()
	utils := fwpb.NewUtilsServiceClient(cl.Conn)

	// Setup servo.
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)
	svo := pxy.Servo()

	// Setup Config.
	platformResponse, err := utils.Platform(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Error during Platform: ", err)
	}
	board := strings.ToLower(platformResponse.Board)
	model := strings.ToLower(platformResponse.Model)
	cfg, err := firmware.NewConfig(s.DataPath(firmware.ConfigDir), board, model)
	if err != nil {
		s.Fatal("Error during NewConfig: ", err)
	}

	// Ensure that DUT starts in the initial mode.
	if r, err := utils.CurrentBootMode(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Error during CurrentBootMode at beginning of test: ", err)
	} else if fwCommon.BootModeFromProto[r.BootMode] != modes[0] {
		s.Logf("At start of test, DUT is in %s mode. Rebooting to initial mode %s", fwCommon.BootModeFromProto[r.BootMode], modes[0])
		if err = firmware.RebootToMode(ctx, d, svo, cfg, cl, modes[0]); err != nil {
			s.Fatalf("Failed to reboot to initial mode %s", modes[0])
		}
		s.Log("Reconnecting to RPC")
		if err = testing.Poll(ctx, func(ctx context.Context) error {
			var err error
			cl, err = rpc.Dial(ctx, d, s.RPCHint(), "cros")
			return err
		}, &testing.PollOptions{Timeout: 3 * time.Minute}); err != nil {
			s.Fatal("Reconnecting to RPC after reboot: ", err)
		}
		utils = fwpb.NewUtilsServiceClient(cl.Conn)
		if r, err = utils.CurrentBootMode(ctx, &empty.Empty{}); err != nil {
			s.Fatalf("Error during CurrentBootMode after trying to reboot to %s: %+v", modes[0], err)
		} else if fwCommon.BootModeFromProto[r.BootMode] != modes[0] {
			s.Fatalf("DUT was in %s after trying to set-up initial mode %s", fwCommon.BootModeFromProto[r.BootMode], modes[0])
		}
	}

	// Transition through the boot modes enumerated in ms, verifying boot mode at each step along the way.
	var fromMode, toMode fwCommon.BootMode
	for i := 0; i < len(modes)-1; i++ {
		fromMode, toMode = modes[i], modes[i+1]
		s.Logf("Beginning transition %d: %s -> %s", i, fromMode, toMode)
		if err := firmware.RebootToMode(ctx, d, svo, cfg, cl, toMode); err != nil {
			s.Errorf("Error during transition from %s to %s: %+v", fromMode, toMode, err)
			break
		}
		// Reestablish RPC connection after reboot.
		s.Log("Reconnecting to RPC")
		if err = testing.Poll(ctx, func(ctx context.Context) error {
			var err error
			cl, err = rpc.Dial(ctx, d, s.RPCHint(), "cros")
			return err
		}, &testing.PollOptions{Timeout: 5 * time.Minute}); err != nil {
			s.Fatal("Reconnecting to RPC after reboot: ", err)
		}
		utils = fwpb.NewUtilsServiceClient(cl.Conn)
		if r, err := utils.CurrentBootMode(ctx, &empty.Empty{}); err != nil {
			s.Errorf("Error during CurrentBootMode after transition from %s to %s: %+v", fromMode, toMode, err)
			break
		} else if fwCommon.BootModeFromProto[r.BootMode] != toMode {
			s.Errorf("DUT was in %s after transition from %s to %s", fwCommon.BootModeFromProto[r.BootMode], fromMode, toMode)
			break
		}
	}
}
