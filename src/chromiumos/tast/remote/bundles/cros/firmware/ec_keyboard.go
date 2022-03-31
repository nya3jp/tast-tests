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
	"chromiumos/tast/remote/firmware"
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
		Timeout:      2 * time.Minute,
		ServiceDeps:  []string{"tast.cros.firmware.UtilsService"},
	})
}

const typeTimeout = 250 * time.Millisecond

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

	// Read and discard initial info text.
	func() {
		text := make(chan string)
		go func() {
			defer close(text)
			for scanner.Scan() {
				text <- scanner.Text()
			}
		}()
		for {
			select {
			case <-time.After(1 * time.Second):
				// Time out after 1 second so it doesn't get stuck here.
				s.Log("Finshed reading preamble")
				return
			case _ = <-text:
				continue
			}
		}
	}()

	for key, keyCode := range testKeyMap {
		s.Logf("Pressing key %q, expecting to read keycode %q", key, keyCode)
		if err = readKeyPress(ctx, h, scanner, key, keyCode); err != nil {
			s.Fatal("Failed to read key: ", err)
		}
		// Wait for reading to complete before entering next key to prevent failing previous read.
		if err = testing.Sleep(ctx, typeTimeout); err != nil {
			s.Fatalf("Failed to sleep for %s waiting to type next key", typeTimeout)
		}
	}
}

func readKeyPress(ctx context.Context, h *firmware.Helper, scanner *bufio.Scanner, key, keyCode string) error {
	regex := `Event.*time.*code\s(\d*)\s\(` + keyCode + `\)`
	expMatch := regexp.MustCompile(regex)

	text := make(chan string)
	go func() {
		defer close(text)
		for scanner.Scan() {
			text <- scanner.Text()
		}
	}()
	if err := h.Servo.ECPressKey(ctx, key); err != nil {
		return errors.Wrap(err, "failed to type key")
	}

	for {
		select {
		case <-time.After(typeTimeout):
			return errors.New("did not detect keycode within expected time")
		case out := <-text:
			if match := expMatch.FindStringSubmatch(out); match != nil {
				testing.ContextLog(ctx, "key pressed: ", match)
				return nil
			}
		}
	}
}
