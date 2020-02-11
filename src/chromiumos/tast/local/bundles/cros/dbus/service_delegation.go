// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dbus

import (
	"context"
	"regexp"

	"chromiumos/tast/testing"
	"chromiumos/tast/testutil"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ServiceDelegation,
		Desc: "Checks D-Bus service files for service lifecycle delegation to Upstart",
		Contacts: []string{
			"sarthakkukreti@chromium.org",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func ServiceDelegation(ctx context.Context, s *testing.State) {
	expectedHeader := regexp.MustCompile("\\[D-BUS Service\\]")
	expectedExec := regexp.MustCompile("Exec=(/bin/false|/sbin/start)")
	dbusServiceInstallPath := "/usr/share/dbus-1/system-services"

	var invalidFileList []string

	files, err := testutil.ReadFiles(dbusServiceInstallPath)

	if err != nil {
		s.Fatal("Something failed: ", err)
	}

	for filename, content := range files {
		if expectedHeader.MatchString(content) && !expectedExec.MatchString(content) {
			invalidFileList = append(invalidFileList, filename)
			s.Error("D-Bus service may be exec'ing service directly: ", filename)
		}

	}

	if len(invalidFileList) > 0 {
		s.Fatal("D-Bus services not delegating to Upstart exist on this device")
	}
}
