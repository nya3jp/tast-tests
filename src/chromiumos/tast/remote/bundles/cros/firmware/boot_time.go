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

	"chromiumos/tast/errors"
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
	apBootMax   time.Duration = time.Second
	maxWaitTime time.Duration = 60 * time.Second
)

// BootTime measures the time from EC boot to the first signal that the AP is booting.
func BootTime(ctx context.Context, s *testing.State) {
	ColdBootFinished := regexp.MustCompile(`power state 1 = S5`)
	// TODO(b/172227463): Vary these strings by platform?
	// This means the AP is initalized, but does not mean ChromeOS is booted.
	ApBootFinished := regexp.MustCompile(`HC 0x|Port 80|ACPI query|power state 3 = S0`)
	// YY-mm-dd HH:MM:SS.sss, but only looking at the seconds.millis here
	// See HOST_STRFTIME in src/platform/ec/util/ec3po/console.py
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
	// Set times to invalid values to start.
	var (
		startTime      time.Duration = -1
		coldBootTime   time.Duration = -1
		apBootTime     time.Duration = -1
		uartTime       time.Duration = -1
		rolloverOffset time.Duration
		leftoverLines  string
	)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
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
					if match := ColdBootFinished.FindString(l); match != "" {
						coldBootTime = uartTime + rolloverOffset - startTime
						s.Logf("Cold Boot = %q", match)
					}
				}
				if coldBootTime >= 0 && apBootTime < 0 {
					if match := ApBootFinished.FindString(l); match != "" {
						apBootTime = uartTime + rolloverOffset - startTime - coldBootTime
						s.Logf("Boot = %q", match)
					}
				}
			}
		}
		if coldBootTime < 0 {
			return errors.New("failed to find ColdBootTime in EC Log")
		}
		if apBootTime < 0 {
			return errors.New("failed to find BootTime in EC Log")
		}
		return nil
	}, &testing.PollOptions{Interval: time.Millisecond * 200, Timeout: maxWaitTime}); err != nil {
		s.Error("EC output parsing failed: ", err)
	}
	s.Logf("EC cold boot time: %s", coldBootTime)
	s.Logf("AP Boot time: %s", apBootTime)
	if coldBootTime > coldBootMax {
		s.Errorf("EC boot time = %s; want <=%s", coldBootTime, coldBootMax)
	}
	if apBootTime > apBootMax {
		s.Errorf("Boot time = %s; want <=%s", apBootTime, apBootMax)
	}
}
