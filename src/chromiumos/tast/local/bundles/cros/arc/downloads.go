// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Downloads,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks Downloads integration is working",
		Contacts:     []string{"nya@chromium.org", "arc-eng@google.com", "cros-arc-te@google.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Data:         []string{"capybara.jpg"},
		Attr:         []string{"group:mainline", "group:arc-functional"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraAttr:         []string{"informational"},
		}},
	})
}

func Downloads(ctx context.Context, s *testing.State) {
	const (
		filename    = "capybara.jpg"
		crosPath    = "/home/chronos/user/Downloads/" + filename
		androidPath = "/storage/emulated/0/Download/" + filename
	)

	a := s.FixtValue().(*arc.PreData).ARC

	expected, err := ioutil.ReadFile(s.DataPath(filename))
	if err != nil {
		s.Fatal("Could not read the test file: ", err)
	}

	// In ARCVM, Downloads integration depends on MyFiles mount.
	if err := arc.WaitForARCMyFilesVolumeMountIfARCVMEnabled(ctx, a); err != nil {
		s.Fatal("Failed to wait for MyFiles to be mounted in ARC: ", err)
	}

	// CrOS -> Android
	if err = ioutil.WriteFile(crosPath, expected, 0666); err != nil {
		s.Fatalf("Could not write to %s: %v", crosPath, err)
	}
	actual, err := a.ReadFile(ctx, androidPath)
	if err != nil {
		s.Error("CrOS -> Android failed: ", err)
	} else if !bytes.Equal(actual, expected) {
		s.Error("CrOS -> Android failed: content mismatch")
	}
	if err = os.Remove(crosPath); err != nil {
		s.Fatal("Failed to remove a file: ", err)
	}

	isARCVMEnabled, err := arc.VMEnabled()
	if err != nil {
		s.Fatal("Failed to check whether ARCVM is enabled: ", err)
	}
	if isARCVMEnabled {
		// Current configuration of virtio-fs has a cache sync issue
		// for a short period of time. In order to avoid adb push
		// failures due to this, insert a second of sleep here.
		// TODO(b/171422889): Remove this hack once the cache sync issue
		// is resolved.
		testing.Sleep(ctx, time.Second)
	}

	// Android -> CrOS
	if err = a.WriteFile(ctx, androidPath, expected); err != nil {
		s.Fatalf("Could not write to %s: %v", androidPath, err)
	}
	actual, err = ioutil.ReadFile(crosPath)
	if err != nil {
		s.Error("Android -> CrOS failed: ", err)
	} else if !bytes.Equal(actual, expected) {
		s.Error("Android -> CrOS failed: content mismatch")
	}
	if err = os.Remove(crosPath); err != nil {
		s.Fatal("Failed to remove a file: ", err)
	}
}
