// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"os"
	"os/exec"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MultiNetworking,
		Desc:         "Verifies guest network setup upon physical interface change",
		Contacts:     []string{"taoyl@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
	})
}

func MultiNetworking(ctx context.Context, s *testing.State) {
	const (
		testNetnsName = "test"
		ifName        = "eth99"
		peerIFName    = "peer99"
		brIFName      = "arc_eth99"
		vethIFName    = "veth_eth99"
		//TODO: Currently it's a experience value. Need a perf test among different platforms to have a better understanding of this value.
		waitTime = 8 * time.Second // The time to wait for arc-networkd to set up virtual network after physical network changes.
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
		defer a.Close()
	}

	startARC()

	s.Log("Testing multinet behavior on device addition")

	// Create a virtual interface and verify that corresponding data path is set up.
	// Use a ethernet name template so arc-networkd treats it as an ethernet interface.
	if err := testexec.CommandContext(ctx, "/bin/ip", "netns", "add", testNetnsName).
		Run(testexec.DumpLogOnError); err != nil {
		// Ignore failure here for potential netns already exists case. If it's a legitimate failure it will fail at next step.
		s.Logf("Failed to create test netns %s: %s", testNetnsName, err)
	}
	defer testexec.CommandContext(ctx, "/bin/ip", "netns", "delete", testNetnsName).Run()

	if err := testexec.CommandContext(ctx, "/bin/ip", "link", "add", ifName, "type", "veth", "peer", "name", peerIFName, "netns", testNetnsName).
		Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to create test interface %s: %s", ifName, err)
	}
	defer testexec.CommandContext(ctx, "/bin/ip", "link", "delete", ifName).Run()

	// Give arc-networkd some time to finish configuration.
	testing.Sleep(ctx, waitTime)

	verifyDeviceAdded := func() (firstError error) {
		// Verify bridge and veth created correctly and veth moved to ARC netns.
		if _, err := os.Stat("/sys/class/net/" + brIFName); os.IsNotExist(err) {
			s.Errorf("Bridge %s was not created", brIFName)
			if firstError == nil {
				firstError = err
			}
		}
		if _, err := os.Stat("/sys/class/net/" + vethIFName); os.IsNotExist(err) {
			s.Errorf("Veth interface %s was not created", vethIFName)
			if firstError == nil {
				firstError = err
			}
		}
		if err := arc.BootstrapCommand(ctx, "/system/bin/ip", "link", "show", ifName).Run(); err != nil {
			s.Errorf("Failed verifying interface %s in ARC (%s)", ifName, err)
			if firstError == nil {
				firstError = err
			}
		}

		// Verify forwarding rule set up correctly.
		if err := testexec.CommandContext(ctx, "/sbin/iptables", "-C", "FORWARD", "-o", brIFName, "-j", "ACCEPT").
			Run(testexec.DumpLogOnError); err != nil {
			s.Errorf("Cannot verify iptables -A FORWARD -o %s -j ACCEPT rule: %s", brIFName, err)
			if firstError == nil {
				firstError = err
			}
		}
		if err := testexec.CommandContext(ctx, "/sbin/ip6tables", "-C", "FORWARD", "-i", ifName, "-o", brIFName, "-j", "ACCEPT").
			Run(testexec.DumpLogOnError); err != nil {
			s.Errorf("Cannot verify ip6tables -A FORWARD -i %s -o %s -j ACCEPT rule: %s", ifName, brIFName, err)
			if firstError == nil {
				firstError = err
			}
		}
		if err := testexec.CommandContext(ctx, "/sbin/ip6tables", "-C", "FORWARD", "-i", brIFName, "-o", ifName, "-j", "ACCEPT").
			Run(testexec.DumpLogOnError); err != nil {
			s.Errorf("Cannot verify ip6tables -A FORWARD -i %s -o %s -j ACCEPT rule: %s", brIFName, ifName, err)
			if firstError == nil {
				firstError = err
			}
		}
		return firstError
	}

	if verifyDeviceAdded() != nil {
		s.Fatal("Test failed on device addition")
	}

	s.Log("Testing multinet behavior on ARC restart")

	// Log out to ensure the container is down.
	upstart.RestartJob(ctx, "ui")
	if err := upstart.WaitForJobStatus(ctx, "arc-network-bridge", upstart.StartGoal, upstart.RunningState, upstart.RejectWrongGoal, 30*time.Second); err != nil {
		s.Fatal("arc-network-bridge job failed to start: ", err)
	}
	// Restart ARC.
	startARC()
	// Give arc-networkd some time to finish configuration.
	testing.Sleep(ctx, waitTime)

	if verifyDeviceAdded() != nil {
		s.Fatal("Test failed on ARC restart")
	}

	s.Log("Testing multinet behavior on device deletion")

	// Remove test device
	if err := testexec.CommandContext(ctx, "/bin/ip", "link", "delete", ifName).Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to delete test interface %s: %s", ifName, err)
	}

	// Give arc-networkd some time to finish configuration.
	testing.Sleep(ctx, waitTime)

	// Verify bridge and veth removed correctly.
	if _, err := os.Stat("/sys/class/net/" + brIFName); err == nil {
		s.Errorf("Bridge %s was not removed", brIFName)
	}
	if _, err := os.Stat("/sys/class/net/" + vethIFName); err == nil {
		s.Errorf("Veth interface %s was not removed", vethIFName)
	}

	// Verify iptables forwarding rule removed correctly.
	if err := testexec.CommandContext(ctx, "/sbin/iptables", "-C", "FORWARD", "-o", brIFName, "-j", "ACCEPT").
		Run(testexec.DumpLogOnError); err == nil {
		s.Errorf("iptables -A FORWARD -o %s -j ACCEPT rule is not removed", brIFName)
	} else if _, ok := err.(*exec.ExitError); !ok {
		s.Error("iptables -C returned unexpected error: ", err)
	}
	if err := testexec.CommandContext(ctx, "/sbin/ip6tables", "-C", "FORWARD", "-i", ifName, "-o", brIFName, "-j", "ACCEPT").
		Run(testexec.DumpLogOnError); err == nil {
		s.Errorf("ip6tables -A FORWARD -i %s -o %s -j ACCEPT rule is not removed", ifName, brIFName)
	} else if _, ok := err.(*exec.ExitError); !ok {
		s.Error("ip6tables -C returned unexpected error: ", err)
	}
	if err := testexec.CommandContext(ctx, "/sbin/ip6tables", "-C", "FORWARD", "-i", brIFName, "-o", ifName, "-j", "ACCEPT").
		Run(testexec.DumpLogOnError); err == nil {
		s.Errorf("ip6tables -A FORWARD -i %s -o %s -j ACCEPT rule is not removed", brIFName, ifName)
	} else if _, ok := err.(*exec.ExitError); !ok {
		s.Error("ip6tables -C returned unexpected error: ", err)
	}
}
