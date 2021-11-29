// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/go-cmp/cmp"

	common "chromiumos/tast/common/firmware"
	commonbios "chromiumos/tast/common/firmware/bios"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/remote/firmware/bios"
	"chromiumos/tast/remote/firmware/fixture"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ServoGBBFlags,
		Desc:         "Verifies GBB flags state can be obtained and manipulated via the servo interface",
		Timeout:      8 * time.Minute,
		Contacts:     []string{"cros-fw-engprod@google.com", "jbettis@google.com"},
		Attr:         []string{"group:firmware", "firmware_cr50", "firmware_ccd"},
		SoftwareDeps: []string{"flashrom"},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService"},
		Fixture:      fixture.NormalMode,
		Data:         []string{"fw-config.json"},
	})
}

func dutControl(ctx context.Context, s *testing.State, svo *servo.Servo, commands [][]string) {
	for _, section := range commands {
		for _, cmd := range section {
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
}

type fwConfig struct {
	dutControlOff           [][]string
	dutControlOn            [][]string
	flashExtraFlagsFlashrom []string
	programmer              string
}

// ServoGBBFlags has been tested to pass with Suzy-Q, Servo V4, Servo V4 + ServoMicro in dual V4 mode.
// Verified fail on Servo V4 + ServoMicro w/o dual v4 mode.
// Has not been tested with CCD closed (assumed to fail), nor with C2D2 (assumed to pass).
func ServoGBBFlags(ctx context.Context, s *testing.State) {

	var flashCmds map[string]map[string]fwConfig

	fwConfigRaw, err := s.DataFileSystem().Open("fw-config.json")
	if err != nil {
		s.Fatal("Failed to open fw-config.json")
	}
	dec := json.NewDecoder(fwConfigRaw)
	err = dec.Decode(&flashCmds)
	if err != nil {
		s.Fatal("Failed to Unmarshall fw-config.json")
	}

	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequirePlatform(ctx); err != nil {
		s.Fatal("Failed to require platform: ", err)
	}
	boardFlashCmds, ok := flashCmds[h.Board]
	if !ok {
		s.Logf("Board %q does not have fw-config, using generic", h.Board)
		boardFlashCmds = flashCmds["generic"]
	}

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	if err := h.Servo.RequireCCD(ctx); err != nil {
		s.Fatal("Servo does not have CCD: ", err)
	}

	servoType, err := h.Servo.GetServoType(ctx)
	if err != nil {
		s.Fatal("Failed to get servo type: ", err)
	}

	dualModePattern := regexp.MustCompile(`^(.*)_with_.*_and(_ccd.*)$`)
	if parts := dualModePattern.FindStringSubmatch(servoType); parts != nil {
		// This is a dual mode servo, but we want the ccd flash config
		servoType = parts[1] + parts[2]
	}

	servoFlashCmds := boardFlashCmds[servoType]

	programmer := servoFlashCmds.programmer
	if programmer == "" {
		s.Log("servoFlashCmds does not have programmer configured, using default")
		programmer = "raiden_debug_spi:target=AP,serial=%s"
	}
	s.Logf("Programmer is %s", programmer)

	ccdSerial, err := h.Servo.GetCCDSerial(ctx)
	if err != nil {
		s.Fatal("Failed to get servo serials: ", err)
	}

	if err = h.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Requiring BiosServiceClient: ", err)
	}

	if err = h.Servo.WatchdogRemove(ctx, servo.WatchdogCCD); err != nil {
		s.Fatal("Failed to remove ccd watchdog: ", err)
	}

	s.Log("Getting GBB flags from BiosService")
	old, err := h.BiosServiceClient.GetGBBFlags(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("initial GetGBBFlags failed: ", err)
	}
	s.Log("Current GBB flags: ", old.Set)

	s.Log("Reading fw image over CCD")
	h.DisconnectDUT(ctx) // Some of the dutControl commands will reboot
	dutControl(ctx, s, h.Servo, servoFlashCmds.dutControlOn)
	programmer = fmt.Sprintf(programmer, ccdSerial)
	img, err := bios.NewRemoteImage(ctx, h.ServoProxy, programmer, commonbios.GBBImageSection, servoFlashCmds.flashExtraFlagsFlashrom)
	if err != nil {
		s.Error("Could not read firmware: ", err)
	}
	dutControl(ctx, s, h.Servo, servoFlashCmds.dutControlOff)
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
	if err := h.WaitConnect(ctx); err != nil {
		s.Fatalf("Failed to connect to DUT: %s", err)
	}

	// We need to change some GBB flag, but it doesn't really matter which.
	// Toggle DEV_SCREEN_SHORT_DELAY
	cf = common.GBBToggle(cf, pb.GBBFlag_DEV_SCREEN_SHORT_DELAY)
	sf = common.GBBToggle(sf, pb.GBBFlag_DEV_SCREEN_SHORT_DELAY)
	if err := img.ClearAndSetGBBFlags(cf, sf); err != nil {
		s.Fatal("Failed to toggle GBB flag in image: ", err)
	}

	s.Log("Writing fw image over CCD")
	h.DisconnectDUT(ctx) // Some of the dutControl commands will reboot
	dutControl(ctx, s, h.Servo, servoFlashCmds.dutControlOn)
	if err = bios.WriteRemoteFlashrom(ctx, h.ServoProxy, programmer, img, commonbios.GBBImageSection, servoFlashCmds.flashExtraFlagsFlashrom); err != nil {
		s.Error("count not write flashrom: ", err)
	}
	dutControl(ctx, s, h.Servo, servoFlashCmds.dutControlOff)
	if s.HasError() {
		return
	}

	// Flashrom restarts the dut, so wait for it to boot
	s.Log("Waiting for reboot")
	if err := h.WaitConnect(ctx); err != nil {
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
