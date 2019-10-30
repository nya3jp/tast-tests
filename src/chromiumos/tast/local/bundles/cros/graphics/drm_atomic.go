// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"

	"chromiumos/tast/autocaps"
	"chromiumos/tast/local/bundles/cros/graphics/drm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DRMAtomic,
		Desc: "Verifies that DRM Atomic is indeed supported or not as per the Caps",
		Contacts: []string{
			"mcasas@chromium.org",
			"chromeos-gfx@google.com",
		},
		Attr: []string{"informational"},
	})
}

func DRMAtomic(ctx context.Context, s *testing.State) {
	// Get capabilities computed by autocaps package and extract drm_atomic.
	staticCaps, err := autocaps.Read(autocaps.DefaultCapabilityDir, nil)
	testing.ContextLog(ctx, "Statically-set capabilities: ", staticCaps)

	isDrmAtomicEnabled := staticCaps["drm_atomic"] == autocaps.Yes

	supported, err := drm.IsDRMAtomicSupported()
	if err != nil {
		s.Fatal("Failed to verify drm atomic support: ", err)
	}
	if supported != isDrmAtomicEnabled {
		if supported {
			s.Fatal("DRM atomic should NOT be supported but it is")
		} else {
			s.Fatal("DRM atomic should be supported but it is NOT")
		}
	}
}
