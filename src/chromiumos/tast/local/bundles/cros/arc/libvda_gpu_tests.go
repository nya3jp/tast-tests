// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
		Desc:         "Runs the non-decoding tests targetting libvda's GPU implementation",
		Contacts:     []string{"alexlau@chromium.org", "chromeos-video-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_vm", "chrome"},
		Timeout:      3 * time.Minute,
	})
}

func LibvdaGpuTests(ctx context.Context, s *testing.State) {
	libvda.RunGPUNonDecodeTests(ctx, s)
}
