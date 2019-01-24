// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package selinux

import (
	"os"
	"path/filepath"
	"regexp"

	"chromiumos/tast/testing"
)

// CheckHomeDirectory checks files contexts under /home
func CheckHomeDirectory(s *testing.State) {
	testCases := []struct {
		path    string // path regexp in string
		context string // expected SELinux file contets regex in string
	}{
		// First match wins, so please do NOT sort this list by ASCII order.
		{"/home", "cros_home"},
		{"/home/chronos/user/(Downloads|MyFiles)(/.*)?", "media_rw_data_file"},
		{"/home/chronos/user(/.*)?", "cros_home_shadow_uid_user"},
		{"/home/chronos/u-[0-9a-f]*/(Downloads|MyFiles)(/.*)?", "media_rw_data_file"},
		{"/home/chronos/u-.*", "cros_home_shadow_uid_user"},
		{"/home/chronos(/.*)?", "cros_home_chronos"},
		{"/home/root", "cros_home_root"},
		{"/home/root/[0-9a-f]*/android-data(/.*)?", "^.*"},
		{"/home/root/.*", "cros_home_shadow_uid_root"},
		{"/home/user", "cros_home_user"},
		{"/home/user/[0-9a-f]*/(Downloads|MyFiles)(/.*)?", "media_rw_data_file"},
		{"/home/user/.*", "cros_home_shadow_uid_user"},
		{"/home/.shadow(|/(salt|salt.sum|install_attributes.pb.*))", "cros_home_shadow"},
		{"/home/.shadow/[0-9a-f]*(/[^/]*)?", "cros_home_shadow_uid"},
		{"/home/.shadow/[0-9a-f]*/mount/root/android-data(/.*)?", "^.*"}, // not tested
		{"/home/.shadow/[0-9a-f]*/mount/root(/.*)?", "cros_home_shadow_uid_root"},
		{"/home/.shadow/[0-9a-f]*/mount/user/(Downloads|MyFiles)(/.*)?", "media_rw_data_file"},
		{"/home/.shadow/[0-9a-f]*/mount/user(/.*)?", "cros_home_shadow_uid_user"},
	}
	filepath.Walk("/home", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if !os.IsNotExist(err) {
				s.Errorf("Failed to walk home directory at %q: %v", path, err)
			}
			return nil
		}
		matched := false
		for _, testCase := range testCases {
			pathRegexp, err := regexp.Compile("^" + testCase.path + "$")
			if err != nil {
				s.Errorf("Failed to compile path regexp %q: %v", testCase.path, err)
				continue
			}
			if pathRegexp.MatchString(path) {
				matched = true
				contextRegexp, err := FileContextRegexp(testCase.context)
				if testCase.context[0] == '^' {
					contextRegexp, err = regexp.Compile(testCase.context)
				}
				if err != nil {
					s.Errorf("Failed to compile context regexp %q: %v", testCase.context, err)
				} else if err := checkFileContext(path, contextRegexp); err != nil {
					s.Errorf("Failed file context check for %v: %v", path, err)
				}
				break
			}
		}
		if !matched {
			s.Errorf("Skipped file %q", path)
		}
		return nil
	})
}
