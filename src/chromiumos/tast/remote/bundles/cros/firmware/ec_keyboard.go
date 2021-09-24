// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/pre"
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
		SoftwareDeps: pre.SoftwareDeps,
		ServiceDeps:  pre.ServiceDeps,
		Pre:          pre.NormalMode(),
		Data:         pre.Data,
		Vars:         pre.Vars,
	})
}

const typeTimeout = 1 * time.Millisecond

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
	h := s.PreValue().(*pre.Value).Helper
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
	cmd.Start()

	for key, keyCode := range testKeyMap {
		if err := typeKey(ctx, h, key); err != nil {
			s.Fatal("Failed to type key: ", err)
		}
		if err = readKey(stdout, keyCode, s); err != nil {
			s.Fatal("Failed to read key: ", err)
		}
	}
}

func typeKey(ctx context.Context, h *firmware.Helper, key string) error {
	row, col, err := h.Servo.GetKeyRowCol(key)
	if err != nil {
		return err
	}
	h.Servo.RunECCommand(ctx, fmt.Sprintf("kbpress %d %d 1", col, row))
	// time.Sleep(2 * time.Second) // if delay needed?
	h.Servo.RunECCommand(ctx, fmt.Sprintf("kbpress %d %d 0", col, row))
	return nil
}

func readKey(stdout io.Reader, keyCode string, s *testing.State) error {
	regex := `Event: time \d+\.\d+\, type \d \(EV_KEY\)\, code \d* \(` + keyCode + `\)`
	s.Log("regex: ", regex)
	expMatch := regexp.MustCompile(regex)
	out := ""
	start := time.Now()
	for {
		if time.Since(start) > typeTimeout {
			break
		}
		bytes := make([]byte, 15)
		_, err := stdout.Read(bytes)
		if err != nil {
			s.Fatal("Error reading stdout: ", err)
		}
		out += string(bytes)
		if match := expMatch.FindStringSubmatch(out); match != nil {
			s.Log("key pressed: ", match)
			return nil
		}
	}
	return errors.New("Did not detect keycode within expected time")
}
