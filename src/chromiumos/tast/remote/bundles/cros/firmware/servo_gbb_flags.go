// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
	"fmt"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/firmware/bios"
	pb "chromiumos/tast/services/cros/firmware"

	"github.com/google/go-cmp/cmp"
)

type hasAPFlashOverCCD struct {
	helper *firmware.Helper
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ServoGBBFlags,
		Desc:         "Verifies GBB flags state can be obtained and manipulated via the servo interface",
		Timeout:      8 * time.Minute,
		Contacts:     []string{"cros-fw-engprod@google.com", "jbettis@google.com"},
		Data:         []string{firmware.ConfigFile},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		Vars:         []string{"servo"},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService"},
		HardwareDeps: hwdep.D(servo.CCDDep()),
	})
}

func dutControl(ctx context.Context, s *testing.State, h *firmware.Helper, commands []string) {
	for _, cmd := range commands {
		s.Logf("dut-control %q", cmd)
		parts := strings.SplitN(cmd, ":", 2)
		if len(parts) == 1 {
			if _, err := h.Servo.GetString(ctx, servo.StringControl(cmd)); err != nil {
				s.Fatalf("Could not read servo string %s: %v", cmd, err)
			}
		} else {
			if err := h.Servo.SetString(ctx, servo.StringControl(parts[0]), parts[1]); err != nil {
				s.Fatalf("Could not read servo string %s: %v", cmd, err)
			}
		}
	}
}

func ServoGBBFlags(ctx context.Context, s *testing.State) {
	dut := s.DUT()
	servoSpec, _ := s.Var("servo")
	h := firmware.NewHelper(dut, s.RPCHint(), s.DataPath(firmware.ConfigFile), servoSpec)
	defer h.Close(ctx)

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to create firmware config: ", err)
	}

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	if h.Config.ApFlashCCDProgrammer == "" {
		s.Skip("DUT does not have ApFlashCCDProgrammer configured ", h.Config)
	}
	s.Logf("Programmer is %s", h.Config.ApFlashCCDProgrammer)

	if ccd, err := h.Servo.EnableCCD(ctx); err != nil {
		s.Fatal("Failed to enable CCD: ", err)
	} else if !ccd {
		s.Skip("Servo with CCD required")
	}

	var ccdSerial string
	if serials, err := h.Servo.GetServoSerials(ctx); err != nil {
		s.Fatal("Failed to get servo serials ", err)
	} else {
		var ok bool
		ccdSerial, ok = serials["ccd"]
		if !ok {
			s.Fatalf("Could not get ccd serial number in %v", serials)
		}
		s.Logf("Servo serials: %+v", serials)
	}

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}

	bs := h.BiosServiceClient

	old, err := bs.GetGBBFlags(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("initial GetGBBFlags failed: ", err)
	}
	s.Log("Current GBB flags: ", old.Set)

	dutControl(ctx, s, h, h.Config.ApFlashCCDPreCommands)
	img, err := bios.NewRemoteImage(ctx, h.ServoProxy, fmt.Sprintf(h.Config.ApFlashCCDProgrammer, ccdSerial), bios.GBBImageSection)
	if err != nil {
		s.Fatal("could not read firmware: ", err)
	}
	dutControl(ctx, s, h, h.Config.ApFlashCCDPostCommands)
	cf, sf, err := img.GetGBBFlags()
	if err != nil {
		s.Fatal("could not get GBB flags: ", err)
	}
	ret := pb.GBBFlagsState{Clear: cf, Set: sf}
	s.Log("CDD GBB flags: ", ret.Set)

	if !cmp.Equal(old.Set, ret.Set) {
		s.Fatal("GBB flags from CDD do not match SSH'd GBB flags ", cmp.Diff(old.Set, ret.Set))
	}
	// Flashrom restarts the dut, so wait for it to boot
	if err := dut.WaitConnect(ctx); err != nil {
		s.Fatalf("failed to connect to DUT: %s", err)
	}

	// req := pb.GBBFlagsState{Set: old.Clear, Clear: old.Set}
	// if _, err = bs.ClearAndSetGBBFlags(ctx, &req); err != nil {
	// 	s.Fatal("initial ClearAndSetGBBFlags failed: ", err)
	// }
	// ctxForCleanup := ctx
	// // 150 seconds is a ballpark estimate, adjust as needed.
	// ctx, cancel := ctxutil.Shorten(ctx, 150*time.Second)
	// defer cancel()

	// checker := checkers.New(h)
	// defer func(ctx context.Context) {
	// 	if _, err := bs.ClearAndSetGBBFlags(ctx, old); err != nil {
	// 		s.Fatal("ClearAndSetGBBFlags to restore original values failed: ", err)
	// 	}

	// 	if err := checker.GBBFlags(ctx, *old); err != nil {
	// 		s.Fatal("all flags should have been restored: ", err)
	// 	}
	// }(ctxForCleanup)

	// if err := checker.GBBFlags(ctx, req); err != nil {
	// 	s.Fatal("all flags should have been toggled: ", err)
	// }
}
