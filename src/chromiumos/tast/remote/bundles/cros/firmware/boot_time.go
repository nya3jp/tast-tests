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

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// testParameters contains all the data needed to run a single test iteration.
type testParameters struct {
	apBootRegexp string
	apBootMax    time.Duration
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         BootTime,
		Desc:         "Measures EC boot time",
		Contacts:     []string{"jbettis@chromium.org", "cros-fw-engprod@google.com"},
		Attr:         []string{"group:firmware", "firmware_ec", "firmware_smoke", "firmware_bringup"},
		Fixture:      fixture.NormalMode,
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Params: []testing.Param{
			{
				Name: "x86",
				ExtraHardwareDeps: hwdep.D(hwdep.X86(), hwdep.SkipOnModel(
					"berknip",
					"dirinboz",
					"ezkinil",
					"gumboz",
					"jelboz",
					"jelboz360",
					"morphius",
					"vilboz",
					"vilboz14",
					"vilboz360",
					"woomax",
				)),
				Val: testParameters{
					apBootRegexp: `HC 0x|Port 80|ACPI query|Executing host reboot command`,
					apBootMax:    1500 * time.Millisecond,
				},
			},
			{
				// Zork machines have a waiver for the boot-to-kernel time. go/zork-waiver-b2k
				Name: "zork",
				ExtraHardwareDeps: hwdep.D(hwdep.Model(
					"berknip",
					"dirinboz",
					"ezkinil",
					"gumboz",
					"jelboz",
					"jelboz360",
					"morphius",
					"vilboz",
					"vilboz14",
					"vilboz360",
					"woomax",
				)),
				Val: testParameters{
					apBootRegexp: `HC 0x|Port 80|ACPI query|Executing host reboot command`,
					apBootMax:    2500 * time.Millisecond,
				},
			},
			{
				Name:              "default",
				ExtraHardwareDeps: hwdep.D(hwdep.NoX86()),
				Val: testParameters{
					apBootRegexp: `power state 3 = S0`,
					apBootMax:    1 * time.Second,
				},
			},
		},
	})
}

const (
	coldBootMax time.Duration = time.Second
	maxWaitTime time.Duration = 60 * time.Second
)

// BootTime measures the time from EC boot to the first signal that the AP is booting.
// This test is not a hard rule, what really matters is the platform.BootPerfServerCrosPerf test.
// See go/chromeos-boottime for a proposal to revisit this.
// It appears that this test is not measuring the same thing as "Boot-to-Kernel" or "Boot to chromeball", but should catch a regression in that time.
func BootTime(ctx context.Context, s *testing.State) {
	param := s.Param().(testParameters)
	rebootingStarted := regexp.MustCompile(`Rebooting!`)
	coldBootFinished := regexp.MustCompile(`power state 1 = S5`)
	// This means the AP is initialized, but does not mean ChromeOS is booted.
	apBootFinished := regexp.MustCompile(param.apBootRegexp)
	// YY-mm-dd HH:MM:SS.sss, but only looking at the MM:SS.sss here
	// See HOST_STRFTIME in src/platform/ec/util/ec3po/console.py
	uartAbsoluteTime := regexp.MustCompile(`^\d+-\d+-\d+ \d+:(\d+):(\d+)\.(\d+)`)

	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	s.Log("Capturing EC log")
	if err := h.Servo.SetOnOff(ctx, servo.ECUARTCapture, servo.On); err != nil {
		s.Fatal("Failed to capture EC UART: ", err)
	}
	defer func() {
		if err := h.Servo.SetOnOff(ctx, servo.ECUARTCapture, servo.Off); err != nil {
			s.Fatal("Failed to disable capture EC UART: ", err)
		}
	}()
	// Read the uart stream just to make sure there isn't buffered data.
	if _, err := h.Servo.GetQuotedString(ctx, servo.ECUARTStream); err != nil {
		s.Fatal("Failed to read UART: ", err)
	}
	s.Log("Rebooting EC")
	if err := h.Servo.RunECCommand(ctx, "reboot"); err != nil {
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
		isStarted      bool
		priorMinute    = -1
	)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if lines, err := h.Servo.GetQuotedString(ctx, servo.ECUARTStream); err != nil {
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
				if match := uartAbsoluteTime.FindStringSubmatch(l); match != nil {
					minute, err := strconv.Atoi(match[1])
					if err != nil {
						s.Fatal("Could not parse uart time mins: ", err)
					}
					secs, err := strconv.Atoi(match[2])
					if err != nil {
						s.Fatal("Could not parse uart time secs: ", err)
					}
					millis, err := strconv.Atoi(match[3])
					if err != nil {
						s.Fatal("Could not parse uart time millis: ", err)
					}
					if minute < priorMinute {
						rolloverOffset += time.Hour
					}
					priorMinute = minute
					uartTime = time.Duration(minute)*time.Minute + time.Duration(secs)*time.Second + time.Duration(millis)*time.Millisecond + rolloverOffset
				}
				s.Logf("%s: %q", uartTime, l)
				if match := rebootingStarted.FindString(l); match != "" {
					isStarted = true
					s.Logf("Reboot detected = %q", match)
				}
				if startTime < 0 && uartTime >= 0 && isStarted {
					startTime = uartTime
				}
				if coldBootTime < 0 {
					if match := coldBootFinished.FindString(l); match != "" {
						coldBootTime = uartTime - startTime
						s.Logf("Cold Boot = %q", match)
					}
				}
				if coldBootTime >= 0 && apBootTime < 0 {
					if match := apBootFinished.FindString(l); match != "" {
						apBootTime = uartTime - startTime - coldBootTime
						s.Logf("AP Boot = %q", match)
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
	if apBootTime > param.apBootMax {
		s.Errorf("AP Boot time = %s; want <=%s", apBootTime, param.apBootMax)
	}
	if s.HasError() {
		s.Log("To debug, check the log in $LOGDIR/autoserv_test/servod_*/ec.txt")
	}
}
