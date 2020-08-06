// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"bytes"
	"context"
	"os/exec"
	"os/user"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SafesetidEnsurePolicy,
		Desc: "Runs SafeSetID though various example ID transitions",
		Contacts: []string{
			"thomascedeno@google.com",
			"mortonm@google.com",
		},
		SoftwareDeps: []string{},
		Attr:         []string{"group:mainline", "informational"},
	})
}

// SafesetidEnsurePolicy forks processes as non-root users and ensures the processes
// can change UID to a user that is explicitly allowed in the system-wide allowlist, but no
// other user.
func SafesetidEnsurePolicy(ctx context.Context, s *testing.State) {
	// Fetch kernel version for later runtime check.
	ver, _, err := sysutil.KernelVersionAndArch()
	if err != nil {
		s.Fatal("Failed to get kernel version: ", err)
	}

	// Need to ensure that all the users in our test exist or have been initialized properly.
	err = checkUsersExist()
	if err != nil {
		s.Fatalf("%v", err)
	}

	// Main test starts here, looping through users and testing uid and gid transitions.
	for _, tc := range []struct {
		parent        string
		child         string
		capSETUID     bool
		expectSuccess bool
	}{
		{"cros-disks", "chronos", true, true},
		{"cros-disks", "fuse-exfat", true, true},
		{"cros-disks", "fuse-sshfs", true, true},
		{"cros-disks", "nobody", true, true},
		{"cros-disks", "ntfs-3g", true, true},
		{"cros-disks", "fuse-rar2fs", true, true},
		{"cros-disks", "fuse-zip", true, true},
		{"cros-disks", "chronos", false, false},
		{"cros-disks", "fuse-exfat", false, false},
		{"cros-disks", "fuse-sshfs", false, false},
		{"cros-disks", "nobody", false, false},
		{"cros-disks", "ntfs-3g", false, false},
		{"cros-disks", "fuse-rar2fs", false, false},
		{"cros-disks", "fuse-zip", false, false},

		{"shill", "nobody", true, true},
		{"shill", "openvpn", true, true},
		{"shill", "vpn", true, true},
		{"shill", "syslog", true, true},
		{"shill", "dhcp", true, true},
		{"shill", "dhcp", false, false},
		{"shill", "openvpn", false, false},
		{"shill", "vpn", false, false},
		{"shill", "syslog", false, false},
		{"shill", "nobody", false, false},

		{"cros-disks", "root", true, false},
		{"openvpn", "root", true, false},
		{"shill", "chronos", true, false},
		{"vpn", "root", true, false},
	} {
		err := transitionSetUID(tc.parent, tc.child, tc.capSETUID, tc.expectSuccess, s)
		if err != nil {
			s.Logf(" %v unable to setuid to %v", tc.parent, tc.child)
			s.Fatalf("%v", err)
		}
		// Need to check for kernel version 5.9, as safesetid for GID's relies on functionality from that version.
		if ver.IsOrLater(5, 9) {
			err = transitionSetGID(tc.parent, tc.child, tc.capSETUID, tc.expectSuccess)
			if err != nil {
				s.Logf(" %v unable to setgid to %v", tc.parent, tc.child)
				s.Fatalf("%v", err)
			}
		} else {
			s.Logf("Kernel version is too old for transitionSetGID, (%v), safesetid needs (5,9)", ver)
		}
	}
	//s.Log("Safesetid test successful!")

}

func transitionSetUID(parent, child string, giveCapSetUID, expectSuccess bool, s *testing.State) error {
	var caps string
	if giveCapSetUID {
		caps = "0xc0"
	} else {
		caps = "0x0"
	}
	cmd := exec.Command(
		"/sbin/minijail0",
		"-u",
		parent,
		"-g",
		parent,
		"-c",
		caps,
		"--",
		"/sbin/capsh",
		"--user="+child,
		"--",
		"-c",
		"/usr/bin/whoami")

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()

	if err != nil {
		if expectSuccess {
			return errors.Wrap(err, stderr.String())
		}
		return nil
	}
	if expectSuccess == false {
		return errors.Errorf("%q allowed to transition without permission to %q", parent, child)
	}
	return nil
}

func transitionSetGID(parent, child string, giveCapSetGID, expectSuccess bool) error {
	var caps string
	if giveCapSetGID {
		caps = "0xc0"
	} else {
		caps = "0x0"
	}
	cmd := exec.Command(
		"/sbin/minijail0",
		"-u",
		parent,
		"-g",
		child,
		"-c",
		caps,
		"--",
		"/sbin/capsh",
		"--user="+child,
		"--",
		"-c",
		"/usr/bin/whoami")

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()

	if err != nil {
		if expectSuccess {
			return errors.Wrap(err, stderr.String())
		}
		return nil
	}
	if expectSuccess == false {
		return errors.Wrap(err, stderr.String())
	}
	return nil
}

func checkUsersExist() error {

	//List of all users used in this program
	usersList := []user.User{
		{Name: "cros-disks", Uid: "213", Gid: "213", HomeDir: "/dev/null"},
		{Name: "chronos", Uid: "1000", Gid: "1000", HomeDir: "/home/chronos/user"},
		{Name: "fuse-exfat", Uid: "302", Gid: "302", HomeDir: "/dev/null"},
		{Name: "fuse-sshfs", Uid: "305", Gid: "305", HomeDir: "/dev/null"},
		{Name: "fuse-rar2fs", Uid: "308", Gid: "308", HomeDir: "/dev/null"},
		{Name: "fuse-smbfs", Uid: "307", Gid: "307", HomeDir: "/dev/null"},
		{Name: "fuse-zip", Uid: "309", Gid: "309", HomeDir: "/dev/null"},
		{Name: "ntfs-3g", Uid: "300", Gid: "300", HomeDir: "/dev/null"},
		{Name: "nobody", Uid: "65534", Gid: "65534", HomeDir: "/dev/null"},
		{Name: "vpn", Uid: "212", Gid: "212", HomeDir: "/dev/null"},
		{Name: "openvpn", Uid: "20174", Gid: "20174", HomeDir: "/dev/null"},
		{Name: "syslog", Uid: "202", Gid: "202", HomeDir: "/dev/null"},
		{Name: "dhcp", Uid: "224", Gid: "224", HomeDir: "/dev/null"},
	}

	for _, val := range usersList {
		_, err := user.Lookup(val.Name)
		if err != nil {
			return errors.Wrap(err, val.Name)
		}
	}

	return nil
}

func listExistingUsers() (string, error) {
	// See which UID's are locally available for debug output
	var buf bytes.Buffer
	listOfNames := exec.Command("cat", "/etc/passwd")
	listOfNames.Stdout = &buf
	err := listOfNames.Run()
	if err != nil {
		return "", errors.Wrap(err, "Unable to list existing users")
	}
	return ("SafeSetID, list of UID's: " + buf.String()), nil
}
