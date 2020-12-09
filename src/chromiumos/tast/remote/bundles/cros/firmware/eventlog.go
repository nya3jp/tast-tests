// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/pre"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Eventlog,
		Desc: "Ensure that eventlog is written on boot and suspend/resume",
		Contacts: []string{
			"gredelston@google.com", // Test author
			"cros-fw-engprod@google.com",
		},
		Attr: []string{"group:mainline", "informational", "group:firmware", "firmware_smoke"},
		Data: []string{firmware.ConfigFile},
		HardwareDeps: hwdep.D(
			// Eventlog is broken/wontfix on veyron devices.
			// See http://b/35585376#comment14 for more info.
			hwdep.SkipOnPlatform("veyron_fievel"),
			hwdep.SkipOnPlatform("veyron_tiger"),
		),
		Pre:          pre.NormalMode(),
		ServiceDeps:  []string{"tast.cros.firmware.UtilsService"},
		SoftwareDeps: []string{"crossystem"},
		Vars:         []string{"servo"},
	})
}

const (
	bashDatetimeFmt = "%Y-%m-%d %H:%M:%S"
	goDatetimeFmt   = "2006-01-02 15:04:05"
)

// reportNow returns the current datetime, from the DUT's perspective.
func reportNow(ctx context.Context, r *reporters.Reporter) (time.Time, error) {
	output, err := r.CommandOutput(ctx, "date", fmt.Sprintf("+%s", bashDatetimeFmt))
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(goDatetimeFmt, output)
}

// reportNewEvents returns a list of events, according to mosys, that occurred after a specified time.
func reportNewEvents(ctx context.Context, r *reporters.Reporter, cutoffTime time.Time) ([]string, error) {
	output, err := r.CommandOutputLines(ctx, "mosys", "eventlog", "list")
	if err != nil {
		return []string{}, err
	}
	now, err := reportNow(ctx, r)
	if err != nil {
		return []string{}, err
	}
	var events []string
	// Iterate through the events in reverse order: most recent first
	for i := len(output) - 1; i > 0; i-- {
		line := output[i]
		split := strings.SplitN(line, " | ", 3)
		if len(split) < 3 {
			return []string{}, errors.Errorf("eventlog entry had fewer than 3 ' | ' delimiters: %q", line)
		}
		timestamp, err := time.Parse(goDatetimeFmt, split[1])
		if err != nil {
			return []string{}, errors.Wrap(err, "parsing eventlog entry")
		}
		if timestamp.After(now) {
			return []string{}, errors.Errorf("eventlog entry lies in the future: now = %s; line = %q", now, line)
		}
		if timestamp.Before(cutoffTime) {
			break
		}
		events = append(events, split[2])
	}
	return events, nil
}

// containsReMatch returns true if any element of sArr matches the regexp.
func containsReMatch(sArr []string, re *regexp.Regexp) bool {
	for _, s := range sArr {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

func Eventlog(ctx context.Context, s *testing.State) {
	// Create mode-switcher
	v := s.PreValue().(*pre.Value)
	h := v.Helper
	defer h.Close(ctx)
	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Creating mode switcher: ", err)
	}
	r := h.Reporter

	// Check eventlog behavior on normal boot
	cutoffTime, err := reportNow(ctx, r)
	if err != nil {
		s.Fatal("Reporting time at start of test: ", err)
	}
	if err := ms.ModeAwareReboot(ctx, firmware.WarmReset); err != nil {
		s.Fatal("Error resetting DUT: ", err)
	}
	events, err := reportNewEvents(ctx, r, cutoffTime)
	if err != nil {
		s.Fatal("Gathering events after normal reboot: ", err)
	}
	if !containsReMatch(events, regexp.MustCompile("System boot")) {
		s.Error("No 'System boot' event on normal boot")
	}
	if containsReMatch(events, regexp.MustCompile("Developer Mode|Recovery Mode|Sleep| Wake")) {
		s.Errorf("Incorrect event logged on normal boot: %s", strings.Join(events, ";;"))
	}

	// TODO(gredelston): Test eventlog upon dev->dev reboot
	// TODO(gredelston): Test eventlog upon normal->rec reboot
	// TODO(gredelston): Test eventlog upon rec->normal reboot
	// TODO(gredelston): Test eventlog upon suspend/resume
	// TODO(gredelston): Test eventlog with hardware watchdog
}
