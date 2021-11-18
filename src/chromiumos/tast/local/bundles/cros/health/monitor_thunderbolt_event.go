// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/typec/typecutils"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MonitorThunderboltEvent,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Monitors the Thunderbolt event detected proper or not after plug/unplug Thunderbolt devices",
		Contacts: []string{"pathan.jilani@intel.com",
			"cros-tdm-tpe-eng@google.com",
			"intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.Model("brya")),
		Fixture:      "crosHealthdRunning",
	})
}

func MonitorThunderboltEvent(ctx context.Context, s *testing.State) {
	var (
		deviceRemoved          = regexp.MustCompile(`Device removed`)
		deviceAdded            = regexp.MustCompile(`Device added`)
		outFile                = "/tmp/tbt_logs.txt"
		thundeBoltMonitorEvent = "sudo nohup cros-health-tool event --category=thunderbolt --length_seconds=600 > " + outFile + " 2>&1 &"
		pidCmd                 = "ps -aux | grep -i nohup | awk -F' ' '{print $2}' | head -1"
		killTbtEvent           = "kill -9 $(" + pidCmd + ")"
		timeOut                = 30 * time.Second
	)

	port, err := typecutils.CheckPortsForTBTPartner(ctx)
	if err != nil {
		s.Fatal("Failed to determine Thunderbolt device from PD identity: ", err)
	}
	s.Logf("Thunderbolt Port is: %d", port)
	if port == -1 {
		s.Fatal("Failed no Thunderbolt device connected to DUT")
	}
	portStr := strconv.Itoa(port)
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	// Run Cmd in background.
	if err := testexec.CommandContext(ctx, "sh", "-c", thundeBoltMonitorEvent).Run(); err != nil {
		s.Fatal("Failed to run monitor event: ", err)
	}
	getThunderBoltEventOutput := func() string {
		output, err := ioutil.ReadFile(outFile)
		if err != nil {
			s.Fatal("Failed to read data: ", err)
		}
		return string(output)
	}

	if err := testexec.CommandContext(ctx, "ectool", "pdcontrol", "suspend", portStr).Run(); err != nil {
		s.Fatal("Failed to simulate unplug: ", err)
	}
	defer func() {
		// Delete tmp file.
		os.RemoveAll(outFile)
		if err := testexec.CommandContext(ctxForCleanUp, "sh", "-c", killTbtEvent).Run(); err != nil {
			s.Log("Failed to kill command thundeBoltMonitorEvent execution: ", err)
		}
		if err := testexec.CommandContext(ctxForCleanUp, "ectool", "pdcontrol", "resume", portStr).Run(); err != nil {
			s.Log("Failed to perform replug: ", err)
		}
	}()

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if !deviceRemoved.MatchString(getThunderBoltEventOutput()) {
			return errors.New("failed to detect deviceRemoved TBT Event")
		}
		return nil

	}, &testing.PollOptions{Timeout: timeOut}); err != nil {
		s.Fatal("Failed to verify no Thunderbolt devices connected after unplug: ", err)
	}
	if err := os.Truncate(outFile, 0); err != nil {
		s.Fatal("Failed to truncate: ", err)
	}
	if err := testexec.CommandContext(ctx, "ectool", "pdcontrol", "resume", portStr).Run(); err != nil {
		s.Fatal("Failed to simulate replug: ", err)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if !(deviceAdded.MatchString(getThunderBoltEventOutput())) {
			return errors.New("failed to detect deviceAdded TBT Event")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeOut}); err != nil {
		s.Fatal("Failed to verify Thunderbolt devices connected after plug: ", err)
	}

}
