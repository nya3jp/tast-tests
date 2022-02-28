// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MiniVM,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Ensures mini-ARCVM is functional and can be upgraded successfully",
		Contacts: []string{
			"wvk@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_vm", "chrome"},
		Timeout:      5 * time.Minute,
		VarDeps:      []string{"ui.gaiaPoolDefault"},
	})
}

func MiniVM(ctx context.Context, s *testing.State) {

	// Setup Chrome and login as an opt-out user. mini-ARCVM should
	// automatically start.
	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	s.Log("Waiting for crosvm to start")
	if err := waitForCrosvmProcess(ctx); err != nil {
		s.Fatal("Failed to wait for crosvm process to start: ", err)
	}

	// Get mini-VM CID from Concierge.
	cid, err := getMiniVMCID(ctx)
	if err != nil {
		s.Fatal("Failed to get mini-VM CID: ", err)
	}

	// Check for init process inside the guest.
	s.Log("Checking for init process in ARCVM")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return vmCommand(ctx, cid, "pidof", "-s", "init").Run()
	}, nil); err != nil {
		s.Fatal("Failed to find init process in guest")
	}

	s.Log("Checking that arcvm-boot-notification-client is running")
	const serviceProp = "init.svc.arcvm-boot-notification-client"
	if err := waitForProp(ctx, cid, serviceProp, "running"); err != nil {
		s.Fatal("Failed to check status of arcvm-boot-notification-client: ", err)
	}

	s.Log("Checking that /data is not mounted")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := vmCommand(ctx, cid, "mountpoint", "-q", "/data").Run(); err == nil {
			return errors.New("/data is mounted")
		}
		return nil
	}, nil); err != nil {
		s.Fatal("Failed to check that /data is not mounted: ", err)
	}

	s.Log("Upgrading the mini-ARCVM instance")
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		s.Fatal("Unable to perform ARC optin: ", err)
	}
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to wait for ARC to finish booting: ", err)
	}
	defer a.Close(ctx)

	s.Log("Checking that arcvm-boot-notification-client is stopped")
	if err := waitForProp(ctx, cid, serviceProp, "stopped"); err != nil {
		s.Fatal("Failed to check status of arcvm-boot-notification-client: ", err)
	}

	s.Log("Checking that /data is mounted")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return vmCommand(ctx, cid, "mountpoint", "-q", "/data").Run()
	}, nil); err != nil {
		s.Fatal("Failed to check /data mount: ", err)
	}

	s.Log("Checking that upgrade props are set")
	// Check a subset of upgrade props. Most of them are not readable from shell
	// due to SELinux policy.
	for _, prop := range []string{
		"ro.boot.arc_demo_mode",
		"ro.boot.enable_adb_sideloading",
	} {
		if err := waitForPropToExist(ctx, cid, prop); err != nil {
			s.Fatalf("Failed to check upgrade prop %s: %v", prop, err)
		}
	}
}

// vmCommand creates a command to be run in the VM over vsh. We cannot use
// android-sh while mini-VM is running since it depends on having the
// cryptohome-ID set for the VM (the ID is not set until the VM is upgraded).
func vmCommand(ctx context.Context, cid int, command string, args ...string) *testexec.Cmd {
	params := []string{"--user=root", "--cid=" + strconv.Itoa(cid), "--", command}
	params = append(params, args...)
	cmd := testexec.CommandContext(ctx, "vsh", params...)
	cmd.Stdin = &bytes.Buffer{}
	return cmd
}

// waitForCrosvmProcess waits for the crosvm process to start. We cannot use
// arc.WaitAndroidInit() here since that uses android-sh.
func waitForCrosvmProcess(ctx context.Context) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		_, err := arc.InitPID()
		return err
	}, nil)
}

// waitForProp polls until prop equals expected, or the context deadline is
// hit.
func waitForProp(ctx context.Context, cid int, prop, expected string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		out, err := vmCommand(ctx, cid, "getprop", prop).Output()
		if err != nil {
			return err
		}
		val := strings.TrimSpace(string(out))
		if val != expected {
			return errors.Errorf("unexpected %s, got: %q; want: %q", prop, val, expected)
		}
		return nil
	}, nil)
}

// waitForPropToExist polls until prop is set.
func waitForPropToExist(ctx context.Context, cid int, prop string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		out, err := vmCommand(ctx, cid, "getprop", prop).Output()
		if err != nil {
			return err
		}
		val := strings.TrimSpace(string(out))
		if len(val) == 0 {
			return errors.Errorf("%s prop is not set", prop)
		}
		return nil
	}, nil)
}

// getMiniVMCID returns the context identifier (CID) of the currently running
// mini-ARCVM instance, if any.
func getMiniVMCID(ctx context.Context) (int, error) {
	out, err := testexec.CommandContext(
		ctx, "concierge_client", "--get_vm_cid", "--name=arcvm",
		"--cryptohome_id=ARCVM_DEFAULT_OWNER").Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, err
	}

	cid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, err
	}
	return cid, nil
}
