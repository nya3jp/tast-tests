// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     CrosConfigFS,
		Desc:     "Check functionality of cros_config to mount ConfigFS",
		Contacts: []string{"jrosenth@chromium.org"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

const configFSImage = "/usr/share/chromeos-config/configfs.img"

func CrosConfigFS(ctx context.Context, s *testing.State) {
	mountpoint, err := ioutil.TempDir("", "tast.platform.CrosConfigFS")
	if err != nil {
		s.Fatal("Failed to make temporary directory for mountpoint: ", err)
	}
	defer os.RemoveAll(mountpoint)

	_, err = os.Stat(configFSImage)
	var args []string
	if os.IsNotExist(err) {
		// This is a non-unibuild board. Use the fallback mount.
		args = []string{"--mount_fallback", mountpoint}
	} else {
		// This is a unibuild board. Mount the configfs image.
		args = []string{"--mount", configFSImage, mountpoint}
	}

	err = testexec.CommandContext(ctx, "cros_config", args...).Run()
	if err != nil {
		s.Fatal("Failed to mount ChromeOS ConfigFS")
	}

	privateDir := filepath.Join(mountpoint, "private")
	v1Dir := filepath.Join(mountpoint, "v1")

	defer testexec.CommandContext(ctx, "umount", privateDir).Run()
	defer testexec.CommandContext(ctx, "umount", v1Dir).Run()

	// The purpose of this test is to make sure the mount
	// succeeds. The unit tests in cros_config_host already check
	// the filesystem contents are correct for unibuild, and the
	// unit tests in libcros_config test probing works for
	// unibuild, and the structure is correct for non-unibuild. So
	// spot-checking that a few (required) files exist should be
	// more than sufficient for this test.
	for _, key := range []string{
		"/name",
		"/brand-code",
		"/identity/platform-name",
	} {
		_, err := os.Stat(filepath.Join(v1Dir, key))
		if err != nil {
			s.Error("Required key does not exist: ", key)
		}
	}
}
