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

// CheckHomeDirectory checks files contexts under /home.
// This contains functionality shared between security.SELinuxFilesARC and
// security.SELinuxFilesNonARC tests.
func CheckHomeDirectory(s *testing.State) {
	const (
		skipTest = "^.*"
	)
	testCases := []struct {
		// path regexp; encapsulated in '^' and '$'.
		path string
		// context is a regular expression matching the expected context.
		// If it is not prefixed with "^", it will be automatically prefixed
		// with "^u:object_r:" and suffixed with "^:s0$" (see FileContextRegexp).
		context string
	}{
		// First match wins, so please do NOT sort this list by ASCII order.
		{`/home`, `cros_home`},
		{`/home/chronos/user/(Downloads|MyFiles)(/.*)?`, `(media_rw_data_file|cros_downloads_file)`},
		// Not logged in users doesn't have real data bind-mounted (cros_home_chronos).
		{`/home/chronos/user(/.*)?`, `(cros_home_shadow_uid_user|cros_home_chronos)`},
		{`/home/chronos/u-[0-9a-f]*/(Downloads|MyFiles)(/.*)?`, `(media_rw_data_file|cros_downloads_file)`},
		// Not logged in users doesn't have real data bind-mounted (cros_home_chronos).
		{`/home/chronos/u-.*`, `(cros_home_shadow_uid_user|cros_home_chronos)`},
		{`/home/chronos(/.*)?`, `cros_home_chronos`},
		{`/home/root`, `cros_home_root`},
		{`/home/root/[0-9a-f]*/android-data(/.*)?`, skipTest},
		{`/home/root/[0-9a-f]*/authpolicyd(/.*)?`, `cros_home_shadow_uid_root_authpolicyd`},
		{`/home/root/[0-9a-f]*/chaps(/.*)?`, `cros_home_shadow_uid_root_chaps`},
		{`/home/root/[0-9a-f]*/session_manager(/.*)?`, `cros_home_shadow_uid_root_session_manager`},
		{`/home/root/[0-9a-f]*/shill(/.*)?`, `cros_home_shadow_uid_root_shill`},
		{`/home/root/[0-9a-f]*/shill_logs(/.*)?`, `cros_home_shadow_uid_root_shill_logs`},
		{`/home/root/[0-9a-f]*/usb_bouncer(/.*)?`, `cros_home_shadow_uid_root_usb_bouncer`},
		// Not logged in users doesn't have real data bind-mounted (cros_home_root).
		{`/home/root/.*`, `(cros_home_shadow_uid_root|cros_home_root)`},
		{`/home/user`, `cros_home_user`},
		{`/home/user/[0-9a-f]*/(Downloads|MyFiles)(/.*)?`, `(media_rw_data_file|cros_downloads_file)`},
		// Not logged in users doesn't have real data bind-mounted (cros_home_user).
		{`/home/user/.*`, `(cros_home_shadow_uid_user|cros_home_user)`},
		{`/home/\.shadow(|/(salt|salt\.sum|install_attributes\.pb.*|\.can_attempt_ownership))`, `cros_home_shadow`},
		{`/home/\.shadow/[0-9a-f]*(/[^/]*)?`, `cros_home_shadow_uid`},
		{`/home/\.shadow/low_entropy_creds(/.*)?`, `cros_home_shadow_low_entropy_creds`},
		// Other unhandled files in .shadow should be cros_home_shadow.
		{`/home/\.shadow/[^/]*`, `cros_home_shadow`},
		{`/home/\.shadow/[0-9a-f]*/mount/root/android-data(/.*)?`, skipTest},
		{`/home/\.shadow/[0-9a-f]*/mount/root/authpolicyd(/.*)?`, `cros_home_shadow_uid_root_authpolicyd`},
		{`/home/\.shadow/[0-9a-f]*/mount/root/chaps(/.*)?`, `cros_home_shadow_uid_root_chaps`},
		{`/home/\.shadow/[0-9a-f]*/mount/root/session_manager(/.*)?`, `cros_home_shadow_uid_root_session_manager`},
		{`/home/\.shadow/[0-9a-f]*/mount/root/shill(/.*)?`, `cros_home_shadow_uid_root_shill`},
		{`/home/\.shadow/[0-9a-f]*/mount/root/shill_logs(/.*)?`, `cros_home_shadow_uid_root_shill_logs`},
		{`/home/\.shadow/[0-9a-f]*/mount/root/usb_bouncer(/.*)?`, `cros_home_shadow_uid_root_usb_bouncer`},
		{`/home/\.shadow/[0-9a-f]*/mount/root(/.*)?`, `cros_home_shadow_uid_root`},
		{`/home/\.shadow/[0-9a-f]*/mount/user/(Downloads|MyFiles)(/.*)?`, `(media_rw_data_file|cros_downloads_file)`},
		{`/home/\.shadow/[0-9a-f]*/mount/user(/.*)?`, `cros_home_shadow_uid_user`},
		// Not logged in users are not decrypted. Skip it.
		{`/home/\.shadow/[0-9a-f]*/mount/.*`, skipTest},
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
			pathRegexp := regexp.MustCompile("^" + testCase.path + "$")
			if !pathRegexp.MatchString(path) {
				continue
			}
			matched = true
			var contextRegexp *regexp.Regexp
			if testCase.context[0] == '^' {
				contextRegexp = regexp.MustCompile(testCase.context)
			} else {
				if contextRegexp, err = FileContextRegexp(testCase.context); err != nil {
					s.Errorf("Failed to compile context regexp %q: %v", testCase.context, err)
					break
				}
			}
			if err := checkFileContext(path, contextRegexp); err != nil {
				s.Errorf("Failed file context check for %v: %v", path, err)
			}
			break
		}
		if !matched {
			s.Error("Unhandled file", path)
		}
		return nil
	})
}
