// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BootTime,
		Desc:         "Measures EC boot time",
		Contacts:     []string{"jbettis@chromium.org", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		Vars:         []string{"servo"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

const (
	coldBootMax time.Duration = time.Second
	bootMax     time.Duration = time.Second
	maxWaitTime time.Duration = 60 * time.Second
)

// BootTime measures the time from EC boot to the first signal that the AP is booting.
func BootTime(ctx context.Context, s *testing.State) {
	ColdBootFinished := regexp.MustCompile(`power state 1 = S5`)
	// TODO(b/172227463): Vary these strings by platform?
	BootFinished := regexp.MustCompile(`HC 0x|Port 80|ACPI query|power state 3 = S0`)
	UARTAbsoluteTime := regexp.MustCompile(`^\d+-\d+-\d+ \d+:\d+:(\d+)\.(\d+)`)

	dut := s.DUT()

	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)
	svo := pxy.Servo()

	s.Log("Capturing EC log")
	if err := svo.SetOnOff(ctx, servo.ECUARTCapture, servo.On); err != nil {
		s.Fatal("Failed to capture EC UART: ", err)
	}
	defer func() {
		if err := svo.SetOnOff(ctx, servo.ECUARTCapture, servo.Off); err != nil {
			s.Fatal("Failed to disable capture EC UART: ", err)
		}
	}()
	s.Log("Rebooting EC")
	if err := svo.RunECCommand(ctx, "reboot"); err != nil {
		s.Fatal("Failed to send reboot command: ", err)
	}
	start := time.Now()
	var startTime time.Duration = -1
	var coldBootTime time.Duration = -1
	var bootTime time.Duration = -1
	var uartTime time.Duration = -1
	var rolloverOffset time.Duration
	var leftoverLines string
	for ; (coldBootTime < 0 || bootTime < 0) && time.Since(start) < maxWaitTime; testing.Sleep(time.Millisecond * 200) {
		if lines, err := svo.GetQuotedString(ctx, servo.ECUARTStream); err != nil {
			s.Fatal("Failed to read UART: ", err)
		} else if lines != "" {
			// It is possible to read partial lines, so save the part after newline for later
			lines = leftoverLines + lines
			if crlfIdx := strings.LastIndex(lines, "\r\n"); crlfIdx < 0 {
				leftoverLines = lines
				lines = ""
			} else {
				leftoverLines = lines[crlfIdx+2:]
				lines = lines[:crlfIdx+2]
			}

			for _, l := range strings.Split(lines, "\r\n") {
				// If the line starts with a timestamp, capture it. Only look at the
				// seconds & millis to make things easier.
				if match := UARTAbsoluteTime.FindStringSubmatch(l); match != nil {
					secs, err := strconv.Atoi(match[1])
					if err != nil {
						s.Fatal("Could not parse uart time secs: ", err)
					}
					millis, err := strconv.Atoi(match[2])
					if err != nil {
						s.Fatal("Could not parse uart time millis: ", err)
					}
					newTime := time.Duration(secs)*time.Second + time.Duration(millis)*time.Millisecond
					if newTime < uartTime {
						rolloverOffset += time.Minute
					}
					uartTime = newTime
				}
				if startTime < 0 && uartTime >= 0 {
					startTime = uartTime + rolloverOffset
				}
				if coldBootTime < 0 {
					if match := ColdBootFinished.FindStringSubmatch(l); match != nil {
						coldBootTime = uartTime + rolloverOffset - startTime
						testing.ContextLogf(ctx, "Cold Boot = %q", match)
					}
				}
				if coldBootTime >= 0 && bootTime < 0 {
					if match := BootFinished.FindStringSubmatch(l); match != nil {
						bootTime = uartTime + rolloverOffset - startTime - coldBootTime
						testing.ContextLogf(ctx, "Boot = %q", match)
					}
				}
			}
		}
	}
	if coldBootTime < 0 {
		s.Error("Never found ColdBootFinished in EC log")
	}
	if bootTime < 0 {
		s.Error("Never found BootFinished in EC log")
	}
	s.Logf("EC cold boot time: %s", coldBootTime)
	s.Logf("Boot time: %s", bootTime)
	if coldBootTime > coldBootMax {
		s.Errorf("EC boot time must be less than %s, but was %s", coldBootMax, coldBootTime)
	}
	if bootTime > bootMax {
		s.Errorf("Boot time must be less than %s, but was %s", bootMax, coldBootTime)
	}
}
