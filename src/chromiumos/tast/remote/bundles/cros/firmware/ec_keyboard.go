// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"bufio"
	"context"
	"regexp"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECKeyboard,
		Desc:         "Test EC Keyboard interface",
		Contacts:     []string{"tij@google.com", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Keyboard()),
		Fixture:      fixture.NormalMode,
		ServiceDeps:  []string{"tast.cros.firmware.UtilsService"},
	})
}

const typeTimeout = 10 * time.Millisecond

var testKeyMap = map[string]string{
	"0":        "KEY_0",
	"b":        "KEY_B",
	"e":        "KEY_E",
	"o":        "KEY_O",
	"r":        "KEY_R",
	"s":        "KEY_S",
	"t":        "KEY_T",
	"<enter>":  "KEY_ENTER",
	"<ctrl_l>": "KEY_LEFTCTRL",
	"<alt_l>":  "KEY_LEFTALT",
}

func ECKeyboard(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}

	if err := h.RequireRPCUtils(ctx); err != nil {
		s.Fatal("Requiring RPC utils: ", err)
	}

	res, err := h.RPCUtils.FindPhysicalKeyboard(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("During FindPhysicalKeyboard: ", err)
	}
	device := res.Path
	s.Log("Device path: ", device)
	cmd := h.DUT.Conn().CommandContext(ctx, "evtest", device)
	stdout, err := cmd.StdoutPipe()
	scanner := bufio.NewScanner(stdout)
	cmd.Start()

	for key, keyCode := range testKeyMap {
		if err := h.Servo.SendKeyPress(ctx, key); err != nil {
			s.Fatal("Failed to type key: ", err)
		}
		if err = readKeyPress(scanner, keyCode, s); err != nil {
			s.Fatal("Failed to read key: ", err)
		}
	}
}

func readKeyPress(scanner *bufio.Scanner, keyCode string, s *testing.State) error {
	regex := `Event.*time.*code\s(\d*)\s\(` + keyCode + `\)`
	expMatch := regexp.MustCompile(regex)

	text := make(chan string)
	go func() {
		for scanner.Scan() {
			text <- scanner.Text()
		}
		close(text)
	}()
	for {
		select {
		case <-time.After(typeTimeout):
			return errors.New("did not detect keycode within expected time")
		case out := <-text:
			if match := expMatch.FindStringSubmatch(out); match != nil {
				s.Log("key pressed: ", match)
				return nil
			}
		}
	}
}
