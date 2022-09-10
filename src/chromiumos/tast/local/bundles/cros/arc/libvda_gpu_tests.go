// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/arc/libvda"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LibvdaGpuTests,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Runs the non-decoding tests targetting libvda's GPU implementation",
		Contacts:     []string{"alexlau@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		// "no_qemu" disables the test on betty. b/168566159#comment3
		SoftwareDeps: []string{"android_vm", "chrome", "no_qemu"},
		Timeout:      4 * time.Minute,
	})
}

func LibvdaGpuTests(ctx context.Context, s *testing.State) {
	libvda.RunGPUNonDecodeTests(ctx, s)
}
