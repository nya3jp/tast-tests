// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"io/ioutil"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SELinuxSanity,
		Desc:         "Checks some SELinux status",
		Contacts:     []string{"fqj@chromium.org", "kroot@chromium.org", "chromeos-security@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"selinux"},
	})
}

func SELinuxSanity(ctx context.Context, s *testing.State) {
	assertFileContent := func(path string, expected string) {
		actual, err := ioutil.ReadFile(path)
		if err != nil {
			s.Errorf("failed to read %q: %v", path, err)
			return
		}
		if string(actual) != expected {
			s.Errorf("expecting %q in %q, but got %q", expected, path, actual)
			return
		}
		s.Logf("%q has expected value %q", path, expected)
	}
	assertFileContent("/sys/fs/selinux/enforce", "1")
	assertFileContent("/sys/fs/selinux/deny_unknown", "1")
	assertFileContent("/sys/fs/selinux/policy_capabilities/nnp_nosuid_transition", "1")
	assertFileContent("/proc/self/attr/current", "u:r:cros_ssh_session:s0\x00")
	assertFileContent("/proc/1/attr/current", "u:r:cros_init:s0\x00")
}
