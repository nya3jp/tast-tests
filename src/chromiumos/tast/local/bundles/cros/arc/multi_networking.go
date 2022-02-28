// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"os"
	"os/exec"
	"time"

	"chromiumos/tast/common/testexec"
	upstartcommon "chromiumos/tast/common/upstart"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MultiNetworking,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Verifies guest network setup upon physical interface change",
		Contacts:     []string{"taoyl@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func MultiNetworking(ctx context.Context, s *testing.State) {
	const (
		testNetnsName                    = "test"
		ifName                           = "eth99"
		peerIFName                       = "peer99"
		brIFName                         = "arc_eth99"
		vethIFName                       = "veth_eth99"
		networkInitializationPollTimeout = 10 * time.Second // The time to wait for patchpaneld to set up virtual network after physical network changes.
		configurationPollTimeout         = 1 * time.Second  // The time to wait for remaining configurations after detecting virtual network
	)

	startARC := func() {
		cr, err := chrome.New(ctx, chrome.ARCEnabled())
		if err != nil {
			s.Fatal("Failed to connect to Chrome: ", err)
		}
		defer cr.Close(ctx)
		a, err := arc.New(ctx, s.OutDir())
		if err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}
		defer a.Close(ctx)
	}

	startARC()

	s.Log("Testing multinet behavior on device addition")

	// Create a virtual interface and verify that corresponding data path is set up.
	// Use a ethernet name template so patchpaneld treats it as an ethernet interface.
	if err := testexec.CommandContext(ctx, "/bin/ip", "netns", "add", testNetnsName).Run(testexec.DumpLogOnError); err != nil {
		// Ignore failure here for potential netns already exists case. If it's a legitimate failure it will fail at next step.
		s.Logf("Failed to create test netns %s: %s", testNetnsName, err)
	}
	defer testexec.CommandContext(ctx, "/bin/ip", "netns", "delete", testNetnsName).Run()

	if err := testexec.CommandContext(ctx, "/bin/ip", "link", "add", ifName, "type", "veth", "peer", "name", peerIFName, "netns", testNetnsName).Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to create test interface %s: %s", ifName, err)
	}
	defer testexec.CommandContext(ctx, "/bin/ip", "link", "delete", ifName).Run()

	verifyDeviceAdded := func() {
		// Verify bridge and veth created correctly and veth moved to ARC netns.
		testing.Poll(ctx, func(ctx context.Context) error {
			if _, err := os.Stat("/sys/class/net/" + brIFName); os.IsNotExist(err) {
				return errors.Wrapf(err, "bridge %s was not created", brIFName)
			}
			return nil
		}, &testing.PollOptions{Timeout: networkInitializationPollTimeout})

		testing.Poll(ctx, func(ctx context.Context) error {
			if _, err := os.Stat("/sys/class/net/" + vethIFName); os.IsNotExist(err) {
				return errors.Wrapf(err, "veth interface %s was not created", vethIFName)
			}
			return nil
		}, &testing.PollOptions{Timeout: configurationPollTimeout})

		testing.Poll(ctx, func(ctx context.Context) error {
			if err := arc.BootstrapCommand(ctx, "/system/bin/ip", "link", "show", ifName).Run(); err != nil {
				return errors.Wrapf(err, "failed verifying interface %s in ARC (%s)", ifName, err)
			}
			return nil
		}, &testing.PollOptions{Timeout: configurationPollTimeout})

		// Verify forwarding rule set up correctly.
		if err := testexec.CommandContext(ctx, "/sbin/iptables", "-C", "FORWARD", "-o", brIFName, "-j", "ACCEPT", "-w").
			Run(testexec.DumpLogOnError); err != nil {
			s.Fatalf("Cannot verify iptables -A FORWARD -o %s -j ACCEPT -w rule: %s", brIFName, err)
		}
		if err := testexec.CommandContext(ctx, "/sbin/ip6tables", "-C", "FORWARD", "-o", brIFName, "-j", "ACCEPT", "-w").
			Run(testexec.DumpLogOnError); err != nil {
			s.Fatalf("Cannot verify ip6tables -A FORWARD -o %s -j ACCEPT -w rule: %s", brIFName, err)
		}
		if err := testexec.CommandContext(ctx, "/sbin/ip6tables", "-C", "FORWARD", "-i", brIFName, "-j", "ACCEPT", "-w").
			Run(testexec.DumpLogOnError); err != nil {
			s.Fatalf("Cannot verify ip6tables -A FORWARD -i %s -j ACCEPT -w rule: %s", brIFName, err)
		}
	}

	verifyDeviceAdded()

	s.Log("Testing multinet behavior on ARC restart")

	// Log out to ensure the container is down.
	upstart.RestartJob(ctx, "ui")
	if err := upstart.WaitForJobStatus(ctx, "patchpanel", upstartcommon.StartGoal, upstartcommon.RunningState, upstart.RejectWrongGoal, 30*time.Second); err != nil {
		s.Fatal("patchpanel job failed to start: ", err)
	}
	// Restart ARC.
	startARC()

	verifyDeviceAdded()

	s.Log("Testing multinet behavior on device deletion")

	// Remove test device
	if err := testexec.CommandContext(ctx, "/bin/ip", "link", "delete", ifName).Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to delete test interface %s: %s", ifName, err)
	}

	// Verify bridge and veth removed correctly.
	testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat("/sys/class/net/" + brIFName); err == nil {
			return errors.Errorf("bridge %s was not removed", brIFName)
		}
		return nil
	}, &testing.PollOptions{Timeout: networkInitializationPollTimeout})

	testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat("/sys/class/net/" + vethIFName); err == nil {
			return errors.Errorf("veth interface %s was not removed", vethIFName)
		}
		return nil
	}, &testing.PollOptions{Timeout: configurationPollTimeout})

	// Verify iptables forwarding rule removed correctly.
	if err := testexec.CommandContext(ctx, "/sbin/iptables", "-C", "FORWARD", "-i", ifName, "-o", brIFName, "-j", "ACCEPT", "-w").
		Run(testexec.DumpLogOnError); err == nil {
		s.Errorf("iptables -A FORWARD -i %s -o %s -j ACCEPT -w rule is not removed", ifName, brIFName)
	} else if _, ok := err.(*exec.ExitError); !ok {
		s.Error("iptables -C returned unexpected error: ", err)
	}
	if err := testexec.CommandContext(ctx, "/sbin/ip6tables", "-C", "FORWARD", "-i", ifName, "-o", brIFName, "-j", "ACCEPT", "-w").
		Run(testexec.DumpLogOnError); err == nil {
		s.Errorf("ip6tables -A FORWARD -i %s -o %s -j ACCEPT -w rule is not removed", ifName, brIFName)
	} else if _, ok := err.(*exec.ExitError); !ok {
		s.Error("ip6tables -C returned unexpected error: ", err)
	}
	if err := testexec.CommandContext(ctx, "/sbin/ip6tables", "-C", "FORWARD", "-i", brIFName, "-o", ifName, "-j", "ACCEPT", "-w").
		Run(testexec.DumpLogOnError); err == nil {
		s.Errorf("ip6tables -A FORWARD -i %s -o %s -j ACCEPT -w rule is not removed", brIFName, ifName)
	} else if _, ok := err.(*exec.ExitError); !ok {
		s.Error("ip6tables -C returned unexpected error: ", err)
	}
}
