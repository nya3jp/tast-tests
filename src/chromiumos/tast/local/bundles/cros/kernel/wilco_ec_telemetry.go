// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernel

import (
	"bytes"
	"context"
	"os"
	"time"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WilcoECTelemetry,
		Desc: "Checks that telemetry requests to the EC (e.g. hardware temperature or fan state info) work on Wilco devices",
		Contacts: []string{
			"ncrews@chromium.org",       // Test author and EC kernel driver author.
			"chromeos-wilco@google.com", // Possesses some more domain-specific knowledge.
			"chromeos-kernel@google.com",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"wilco"},
		Timeout:      10 * time.Second,
	})
}

// WilcoECTelemetry tests the Wilco EC's ability to respond to telemetry
// commands. The Wilco EC is able to return telemetry information (such
// as temperature and fan state) via sysfs: You write a binary command
// to the sysfs file, the kernel driver performs some filtering on the
// command to ensure it is sane, and the EC returns the binary result,
// to be read from the same file. You must keep the file descriptor open
// between the read and write for the response to be kept. This test
// checks for end-to-end communication with the EC, and checks that the
// driver performs some basic filtering and sanity checks. Since the
// responses are variable and opaque binary data, it's impractical to
// actually check the values of the responses.
//
// See https://chromium.googlesource.com/chromiumos/third_party/kernel/+/ea0f6a09a7a993fc7c781fd8ca675b29c42d4719/drivers/platform/chrome/wilco_ec/telemetry.c
// for the kernel driver.
func WilcoECTelemetry(ctx context.Context, s *testing.State) {
	type errMsg string

	// These are intended to be merely human-readable and may not necessarily
	// match exactly any real error messages.
	const (
		noErr      errMsg = "NONE"
		invalidErr errMsg = "invalid argument"
		tooLongErr errMsg = "message too long"
	)

	type params struct {
		// Telemetry command to run.
		cmd []byte
		// If the command fails, this should be the cause of the error.
		expectedErr errMsg
	}

	const (
		telemPath = "/dev/wilco_telem0"
		// Number of bytes in a telemetry request and response.
		telemetryCommandSize = 32
	)

	// Send the telemetry command to the EC. Return whether the write succeeded.
	writeCommand := func(f *os.File, p params) (success bool) {
		_, err := f.Write(p.cmd)
		// It would be brittle to check for specific errors, so the best we can do
		// is check for the existence of errors.
		if err != nil {
			if p.expectedErr == noErr {
				s.Errorf("Sending command [% #x] failed, but was supposed to succeed: %v", p.cmd, err)
			}
			return false
		}
		if err == nil && p.expectedErr != noErr {
			s.Errorf("Sending command [% #x] succeded, but was supposed to fail with a %q error", p.cmd, p.expectedErr)
			return false
		}
		return true
	}

	readAndCheckResult := func(f *os.File, p params) {
		var bytes [telemetryCommandSize]byte
		n, err := f.Read(bytes[:])
		if err != nil {
			s.Errorf("Failed to read %s after sending command [% #x]: %v", telemPath, p.cmd, err)
			return
		}
		if n != telemetryCommandSize {
			s.Errorf("Read %v bytes from %v after sending command [% #x]; needed to read %v", n, telemPath, p.cmd, telemetryCommandSize)
			return
		}
		// The result of telemetry commands is not deterministic, so the best we
		// can do is check that there is at least something non-zero in there.
		// I don't *think* the EC ever returns all 0s as a valid response, but
		// perhaps it sometimes does? Please adjust if you encounter flakiness.
		for _, b := range bytes {
			if b != 0 {
				return
			}
		}
		s.Fatalf("Bytes returned from command [% #x] were all zero", p.cmd)
	}

	f, err := os.OpenFile(telemPath, os.O_RDWR, 0644)
	if err != nil {
		s.Fatalf("Failed to open %s: %v", telemPath, err)
	}
	defer f.Close()

	for _, p := range []params{
		// Get various pieces of EC FW version information. For the 3rd byte:
		// 0 = label
		// 1 = svn_rev
		// 2 = model_no
		// 3 = build_date
		{[]byte{0x38, 0x00, 0x00}, noErr},
		{[]byte{0x38, 0x00, 0x01}, noErr},
		{[]byte{0x38, 0x00, 0x02}, noErr},
		{[]byte{0x38, 0x00, 0x03}, noErr},

		// The 2nd byte on all commands is reserved to be 0.
		{[]byte{0x38, 0x01, 0x00}, invalidErr},

		// Too short.
		{[]byte{}, invalidErr},
		// Too long for this command.
		{[]byte{0x38, 0x00, 0x03, 0x00}, tooLongErr},
		// Too long in general. This is 33 bytes, but max is 32.
		{append([]byte{0x38}, bytes.Repeat([]byte{0}, telemetryCommandSize)...), tooLongErr},

		// Bad first byte, not one of the allowed commands. The list of allowed
		// commands are in the kernel driver, linked above.
		{[]byte{0x39, 0x00, 0x03}, invalidErr},

		// Read the temperature from various sensors.
		{[]byte{0x2c, 0x00, 0x00}, noErr},
		{[]byte{0x2c, 0x00, 0x01}, noErr},
		{[]byte{0x2c, 0x00, 0x02}, noErr},
		{[]byte{0x2c, 0x00, 0x03}, noErr},
		// Too many bytes for this command.
		{[]byte{0x2c, 0x00, 0x03, 0x00}, tooLongErr},

		// Get the battery PPID info. 3rd byte should always be 1.
		{[]byte{0x8a, 0x00, 0x01}, noErr},
		{[]byte{0x8a, 0x00, 0x00}, invalidErr},
		{[]byte{0x8a, 0x00, 0x02}, invalidErr},
	} {
		if writeCommand(f, p) {
			readAndCheckResult(f, p)
		}
	}
}
