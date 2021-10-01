// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/remote/firmware/pre"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         S3StabilityCheck,
		Desc:         "Test DUT S3 entry and exit stability check",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome", "crossystem", "flashrom"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService", "tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
		Vars:         pre.Vars,
		Attr:         []string{"group:firmware", "firmware_experimental"},
		Pre:          pre.NormalMode(),
		Data:         pre.Data,
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

func S3StabilityCheck(ctx context.Context, s *testing.State) {
	h := s.PreValue().(*pre.Value).Helper
	var (
		WakeUpFromS3      = regexp.MustCompile("Waking up from system sleep state S3")
		requiredEventSets = [][]string{[]string{`Sleep`, `^Wake`},
			[]string{`ACPI Enter \| S3`, `ACPI Wake \| S3`},
		}
		RestartPowerd  = regexp.MustCompile("powerd start/running")
		SuspndFailure  = regexp.MustCompile("Suspend failures: 0")
		FrmwreLogError = regexp.MustCompile("Firmware log errors: 0")
	)
	const (
		S3DmesgCmd       = "dmesg | grep S3"
		ClrDemsgCmd      = "dmesg -C"
		SwitchToS3Cmd    = "echo 0 > /var/lib/power_manager/suspend_to_idle"
		RestartPowerdCmd = "restart powerd"
		S3MemSleepCmd    = "echo deep > /sys/power/mem_sleep"
		SwitchToS0ixCmd  = "echo 1 > /var/lib/power_manager/suspend_to_idle"
		S0ixMemSleepCmd  = "echo s2idle > /sys/power/mem_sleep"
		PowerdConfigCmd  = "check_powerd_config --suspend_to_idle; echo $?"
		SuspendStressCmd = "suspend_stress_test -c 10"
	)
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)
	client := security.NewBootLockboxServiceClient(cl.Conn)
	if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	r := h.Reporter
	var cutoffEvent reporters.Event
	oldEvents, err := r.EventlogList(ctx)
	if err != nil {
		s.Fatal("Failed finding last event: ", err)
	}
	if len(oldEvents) > 0 {
		cutoffEvent = oldEvents[len(oldEvents)-1]
	}
	if err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", SwitchToS3Cmd).Run(); err != nil {
		s.Fatalf("Failed to execute %s command: %v", SwitchToS3Cmd, err)
	}
	restartOut, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", RestartPowerdCmd).Output()
	if err != nil {
		s.Fatalf("Failed to execute %s command: %v", RestartPowerdCmd, err)
	}
	if !RestartPowerd.MatchString(string(restartOut)) {
		s.Fatal("Failed powerd to restart: ", err)
	}
	if err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", S3MemSleepCmd).Run(); err != nil {
		s.Fatalf("Failed to execute %s command: %v", S3MemSleepCmd, err)
	}
	defer func() {
		if err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", SwitchToS0ixCmd).Run(); err != nil {
			s.Fatalf("Failed to execute %s command: %v", SwitchToS0ixCmd, err)
		}
		restartOut, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", RestartPowerdCmd).Output()
		if err != nil {
			s.Fatalf("Failed to execute %s command: %v", RestartPowerdCmd, err)
		}
		if !RestartPowerd.MatchString(string(restartOut)) {
			s.Fatal("Failed powerd to restart: ", err)
		}
		if err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", S0ixMemSleepCmd).Run(); err != nil {
			s.Fatalf("Failed to execute %s command: %v", S0ixMemSleepCmd, err)
		}
	}()
	if err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", ClrDemsgCmd).Run(); err != nil {
		s.Fatalf("Failed to execute %s command: %v", ClrDemsgCmd, err)
	}
	configValue, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", PowerdConfigCmd).Output()
	if err != nil {
		s.Fatalf("Failed to execute %s command: %v", PowerdConfigCmd, err)
	}
	actualValue := strings.TrimSpace(string(configValue))
	expectedValue := "1"
	if actualValue != expectedValue {
		s.Fatalf("Failed to be in S3 state; expected PowerdConfig value %s; got %s", expectedValue, actualValue)

	}
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}
	stressOut, err := h.DUT.Conn().CommandContext(ctx, "sh", "-c", SuspendStressCmd).Output()
	if err != nil {
		s.Fatal("Failed to execute suspend_stress_test command: ", err)
	}
	var errorCodes []*regexp.Regexp
	errorCodes = []*regexp.Regexp{SuspndFailure, FrmwreLogError}
	for _, errMsg := range errorCodes {
		if !(errMsg).MatchString(string(stressOut)) {
			s.Fatalf("Failed for failures; expected %q but got non-zero %s", errMsg, string(stressOut))
		}
	}
	dmesgOut, err := h.DUT.Conn().CommandContext(ctx, "bash", "-c", S3DmesgCmd).Output()
	if err != nil {
		s.Fatalf("Failed to execute %s command: %v", S3DmesgCmd, err)
	}
	if !WakeUpFromS3.MatchString(string(dmesgOut)) {
		s.Fatalf("Failed to find %q pattern in dmesg log", WakeUpFromS3)
	}
	events, err := r.EventlogListAfter(ctx, cutoffEvent)
	if err != nil {
		s.Fatal("Failed gathering events: ", err)
	}
	requiredEventsFound := false
	for _, requiredEventSet := range requiredEventSets {
		foundAllRequiredEventsInSet := true
		for _, requiredEvent := range requiredEventSet {
			reRequiredEvent := regexp.MustCompile(requiredEvent)
			if !eventMessagesContainMatch(ctx, events, reRequiredEvent) {
				foundAllRequiredEventsInSet = false
				break
			}
		}
		if foundAllRequiredEventsInSet {
			requiredEventsFound = true
			break
		}
	}
	if !requiredEventsFound {
		s.Fatal("Failed as required event missing")
	}
}

func eventMessagesContainMatch(ctx context.Context, events []reporters.Event, re *regexp.Regexp) bool {
	for _, event := range events {
		if re.MatchString(event.Message) {
			return true
		}
	}
	return falses
}
