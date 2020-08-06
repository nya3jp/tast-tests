// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"bytes"
	"context"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

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

func SafesetidEnsurePolicy(ctx context.Context, s *testing.State) {
	/*
	   Forks processes as non-root users and ensures the processes can change UID
	   to a user that is explicitly allowed in the system-wide allowlist, but no
	   other user.
	*/

	//Fetch kernel version for later runtime check
	ver, err := getKernelVersion()
	if err != nil {
		s.Fatal("Failed to get kernel version: ", err)
	}

	//Need to ensure that all the users in our test exist or have been initialized properly
	err = checkUsersExist()
	if err != nil {
		s.Fatalf("%v", err)
	}

	//Main test starts here, looping through users and testing uid and gid transitions
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
		//Need to check for kernel version 5.9, as safesetid for GID's relies on functionality from that version
		if checkKernelVersion(ver, 5, 9) {
			err = transitionSetGID(tc.parent, tc.child, tc.capSETUID, tc.expectSuccess)
			if err != nil {
				s.Logf(" %v unable to setgid to %v", tc.parent, tc.child)
				s.Fatalf("%v", err)
			}
		} else {
			//s.Logf("Kernel version is too old for transitionSetGID, (%v,%v), safesetid needs (5,8)", ver.major, ver.minor)
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
		return errors.New(parent + " allowed to transition without permission to " + child)
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
	//listExistingUsers(s)

	//List of all users used in this program
	userslist := []user.User{
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

	for _, val := range userslist {
		_, err := user.Lookup(val.Name)
		if err != nil {
			return errors.Wrap(err, val.Name)
		}
	}

	//listExistingUsers(s)

	return nil
}

func listExistingUsers(s *testing.State) {
	// See which UID's are locally available for debug output
	var buf bytes.Buffer
	listofnames := exec.Command("cat", "/etc/passwd")
	listofnames.Stdout = &buf
	tmperr := listofnames.Run()
	if tmperr != nil {
		s.Log("Unable to list existing users")
	}
	s.Log("SafeSetID, list of UID's:")
	s.Logf("%v", buf.String())
	return
}

type kernelVersion struct {
	major, minor int
}

func (v *kernelVersion) isOrLater(major, minor int) bool {
	return v.major > major || v.major == major && v.minor >= minor
}

func checkKernelVersion(ver *kernelVersion, major, minor int) bool {

	return ver.isOrLater(major, minor)
}

func getKernelVersion() (*kernelVersion, error) {
	u, err := sysutil.Uname()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get uname")
	}
	t := strings.SplitN(u.Release, ".", 3)
	major, err := strconv.Atoi(t[0])
	if err != nil {
		return nil, errors.Wrapf(err, "wrong release format %q", u.Release)
	}
	minor, err := strconv.Atoi(t[1])
	if err != nil {
		return nil, errors.Wrapf(err, "wrong release format %q", u.Release)
	}
	return &kernelVersion{major: major, minor: minor}, nil
}
