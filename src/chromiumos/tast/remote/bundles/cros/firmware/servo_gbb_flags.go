// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/go-cmp/cmp"

	commonbios "chromiumos/tast/common/firmware/bios"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/bios"
	"chromiumos/tast/remote/firmware/pre"
	"chromiumos/tast/remote/servo"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ServoGBBFlags,
		Desc:         "Verifies GBB flags state can be obtained and manipulated via the servo interface",
		Timeout:      8 * time.Minute,
		Contacts:     []string{"cros-fw-engprod@google.com", "jbettis@google.com"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		Data:         []string{firmware.ConfigFile},
		Pre:          pre.NormalMode(),
		ServiceDeps:  []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		Vars:         []string{"servo"},
	})
}

func dutControl(ctx context.Context, s *testing.State, svo *servo.Servo, commands []string) {
	for _, cmd := range commands {
		s.Logf("dut-control %q", cmd)
		parts := strings.SplitN(cmd, ":", 2)
		if len(parts) == 1 {
			if _, err := svo.GetString(ctx, servo.StringControl(cmd)); err != nil {
				s.Errorf("Could not read servo string %s: %v", cmd, err)
			}
		} else {
			if err := svo.SetString(ctx, servo.StringControl(parts[0]), parts[1]); err != nil {
				s.Errorf("Could not set servo string %s: %v", cmd, err)
			}
		}
	}
}

func toggle(flags []pb.GBBFlag, flag pb.GBBFlag) []pb.GBBFlag {
	var ret []pb.GBBFlag
	found := false
	for _, v := range flags {
		if v == flag {
			found = true
		} else {
			ret = append(ret, v)
		}
	}
	if !found {
		ret = append(ret, flag)
	}
	return ret
}

// ServoGBBFlags has been tested to pass with Suzy-Q, Servo V4, Servo V4 + ServoMicro in dual V4 mode.
// Verified fail on Servo V4 + ServoMicro w/o dual v4 mode.
func ServoGBBFlags(ctx context.Context, s *testing.State) {
	h := s.PreValue().(*pre.Value).Helper

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to create firmware config: ", err)
	}

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	if h.Config.APFlashCCDProgrammer == "" {
		s.Fatal("DUT does not have APFlashCCDProgrammer configured ", h.Config)
	}
	s.Logf("Programmer is %s", h.Config.APFlashCCDProgrammer)

	if err := h.Servo.RequireCCD(ctx); err != nil {
		s.Fatal("Servo does not have CCD: ", err)
	}

	ccdSerial, err := h.Servo.GetCCDSerial(ctx)
	if err != nil {
		s.Fatal("Failed to get servo serials: ", err)
	}

	if err = h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}

	s.Log("Getting GBB flags from BiosService")
	old, err := h.BiosServiceClient.GetGBBFlags(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("initial GetGBBFlags failed: ", err)
	}
	s.Log("Current GBB flags: ", old.Set)

	testing.Sleep(ctx, 1)
	s.Log("Reading fw image over CCD")
	h.DisconnectDUT(ctx) // Some of the dutControl commands will reboot
	dutControl(ctx, s, h.Servo, h.Config.APFlashCCDPreCommands)
	programmer := fmt.Sprintf(h.Config.APFlashCCDProgrammer, ccdSerial)
	img, err := bios.NewRemoteImage(ctx, h.ServoProxy, programmer, commonbios.GBBImageSection)
	if err != nil {
		img, err = bios.NewRemoteImage(ctx, h.ServoProxy, programmer, commonbios.GBBImageSection)
		if err != nil {
			s.Error("Could not read firmware: ", err)
		}
	}
	dutControl(ctx, s, h.Servo, h.Config.APFlashCCDPostCommands)
	if s.HasError() {
		return
	}

	cf, sf, err := img.GetGBBFlags()
	if err != nil {
		s.Fatal("Could not get GBB flags: ", err)
	}
	ret := pb.GBBFlagsState{Clear: cf, Set: sf}
	s.Log("CDD GBB flags: ", ret.Set)

	sortSlice := cmp.Transformer("Sort", func(in []pb.GBBFlag) []pb.GBBFlag {
		out := append([]pb.GBBFlag(nil), in...)
		sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
		return out
	})
	if !cmp.Equal(old.Set, ret.Set, sortSlice) {
		s.Fatal("GBB flags from CDD do not match SSH'd GBB flags ", cmp.Diff(old.Set, ret.Set, sortSlice))
	}
	// Flashrom restarts the dut, so wait for it to boot
	s.Log("Waiting for reboot")
	if err := h.DUT.WaitConnect(ctx); err != nil {
		s.Fatalf("Failed to connect to DUT: %s", err)
	}

	// We need to change some GBB flag, but it doesn't really matter which.
	// Toggle DEV_SCREEN_SHORT_DELAY
	cf = toggle(cf, pb.GBBFlag_DEV_SCREEN_SHORT_DELAY)
	sf = toggle(sf, pb.GBBFlag_DEV_SCREEN_SHORT_DELAY)
	if err := img.ClearAndSetGBBFlags(cf, sf); err != nil {
		s.Fatal("Failed to toggle GBB flag in image: ", err)
	}

	s.Log("Writing fw image over CCD")
	h.DisconnectDUT(ctx) // Some of the dutControl commands will reboot
	dutControl(ctx, s, h.Servo, h.Config.APFlashCCDPreCommands)
	testing.Sleep(ctx, 1)
	if err = bios.WriteRemoteFlashrom(ctx, h.ServoProxy, programmer, img, commonbios.GBBImageSection); err != nil {
		s.Error("count not write flashrom: ", err)
	}
	dutControl(ctx, s, h.Servo, h.Config.APFlashCCDPostCommands)
	if s.HasError() {
		return
	}

	// Flashrom restarts the dut, so wait for it to boot
	s.Log("Waiting for reboot")
	if err := h.DUT.WaitConnect(ctx); err != nil {
		s.Fatalf("Failed to connect to DUT: %s", err)
	}

	if err := h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}

	s.Log("Getting GBB flags from BiosService")
	newFlags, err := h.BiosServiceClient.GetGBBFlags(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("final GetGBBFlags failed: ", err)
	}
	s.Log("Updated GBB flags: ", newFlags.Set)
	expected := pb.GBBFlagsState{Clear: cf, Set: sf}
	if !cmp.Equal(expected.Set, newFlags.Set, sortSlice) {
		s.Fatal("Updated GBB flags do not match SSH'd GBB flags ", cmp.Diff(expected.Set, newFlags.Set, sortSlice))
	}
}
