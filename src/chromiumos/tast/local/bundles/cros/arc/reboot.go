// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

type rebootConfig struct {
	// numTrials is the number of times to repeat reboots in a test.
	numTrials int
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Reboot,
		Desc:         "Checks whether Android can be repeatedly rebooted",
		Contacts:     []string{"youkichihosoi@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Val: rebootConfig{
				numTrials: 3,
			},
			ExtraSoftwareDeps: []string{"android"},
			Timeout:           5 * time.Minute,
			Pre:               arc.Booted(),
		}, {
			Name: "vm",
			Val: rebootConfig{
				numTrials: 3,
			},
			ExtraSoftwareDeps: []string{"android_vm"},
			Timeout:           5 * time.Minute,
			Pre:               arc.VMBooted(),
		}},
	})
}

func Reboot(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome
	numTrials := s.Param().(rebootConfig).numTrials
	for i := 0; i < numTrials; i++ {
		s.Logf("Trial %d/%d", i+1, numTrials)
		if err := runReboot(ctx, s, a, cr); err != nil {
			s.Fatal("Failure: ", err)
		}
	}
}

// runReboot reboots Android and re-establishes ADB connection.
// It assumes that ADB connection has already been established at call-time.
func runReboot(ctx context.Context, s *testing.State, a *arc.ARC, cr *chrome.Chrome) error {
	vm, err := arc.VMEnabled()
	if err != nil {
		return errors.Wrap(err, "failed to determine if ARCVM is enabled")
	}

	oldPID, err := arc.InitPID()
	if err != nil {
		return errors.Wrap(err, "failed to get init PID before reboot")
	}

	oldCID := -1
	if vm {
		oldCID, err = getVMCID(ctx, cr)
		if err != nil {
			return errors.Wrap(err, "failed to get vsock CID of ARCVM before reboot")
		}
	}

	s.Log("Running reboot command via ADB")
	if err := a.Command(ctx, "reboot").Run(); err != nil {
		return errors.Wrap(err, "failed to run reboot command via ADB")
	}

	s.Log("Waiting for old init process to exit")
	if err := waitProcessExit(ctx, oldPID); err != nil {
		return errors.Wrap(err, "failed to wait for old init process to exit")
	}

	s.Log("Waiting for Android boot")
	if err := a.WaitAndroidBoot(ctx, s.OutDir()); err != nil {
		return errors.Wrap(err, "failed to boot Android and re-establish ADB connection")
	}

	newPID, err := arc.InitPID()
	if err != nil {
		return errors.Wrap(err, "failed to get init PID after reboot")
	}
	if newPID == oldPID {
		return errors.New("init PID did not change")
	}

	if vm {
		newCID, err := getVMCID(ctx, cr)
		if err != nil {
			return errors.Wrap(err, "failed to get vsock CID of ARCVM after reboot")
		}
		if newCID == oldCID {
			return errors.New("vsock CID of ARCVM did not change")
		}
	}
	return nil
}

// waitProcessExit waits for a process to exit.
// It checks process state via the function process.NewProcess().
func waitProcessExit(ctx context.Context, pid int32) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		_, err := process.NewProcess(pid)
		if err == nil {
			return errors.Errorf("pid %d still exists", pid)
		}
		return nil
	}, &testing.PollOptions{Timeout: 60 * time.Second})
}

// getVMCID gets vsock CID of ARCVM via concierge_client.
// It assumes that ARCVM is enabled.
func getVMCID(ctx context.Context, cr *chrome.Chrome) (int, error) {
	vm, err := arc.VMEnabled()
	if err != nil {
		return -1, errors.Wrap(err, "failed to determine if ARCVM is enabled")
	} else if !vm {
		return -1, errors.New("ARCVM is not enabled")
	}

	h, err := cryptohome.UserHash(ctx, cr.User())
	if err != nil {
		return -1, errors.Wrapf(err, "failed to get user hash for %q", cr.User())
	}

	cid := -1
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		cmd := testexec.CommandContext(ctx, "concierge_client",
			"--get_vm_cid", "--name=arcvm", "--cryptohome_id="+h)
		o, err := cmd.Output(testexec.DumpLogOnError)
		if err != nil {
			return errors.Wrap(err, "failed to run concierge_client")
		}
		asc := strings.TrimSpace(string(o))
		cid, err = strconv.Atoi(asc)
		if err != nil {
			return errors.Wrapf(err, "failed to covert %q to int", asc)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return -1, errors.Wrap(err, "failed to get vsock CID of ARCVM via concierge_client")
	}
	return cid, nil
}
