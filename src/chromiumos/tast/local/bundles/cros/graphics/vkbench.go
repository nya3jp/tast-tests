// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"time"

	"chromiumos/tast/local/graphics/vkbench"
	"chromiumos/tast/testing"
)

// Tast framework requires every subtest's Val have the same reflect type.
// This is a wrapper to wrap interface together.
type vkConfig struct {
	config vkbench.Config
}

func init() {
	testing.AddTest(&testing.Test{
		Func: VKBench,
		Desc: "Run vkbench (a benchmark that times graphics intensive activities for vulkan), check results and report its performance",
		Contacts: []string{
			"pwang@chromium.org",
			"chromeos-gfx@google.com",
		},
		SoftwareDeps: []string{"no_qemu", "vulkan"},
		Params: []testing.Param{{
			Name:      "",
			Val:       vkConfig{config: &vkbench.CrosConfig{}},
			ExtraAttr: []string{"group:mainline", "informational", "group:graphics", "graphics_nightly"},
			Fixture:   "graphicsNoChrome",
			Timeout:   10 * time.Minute,
		}, {
			Name:      "hasty",
			Val:       vkConfig{config: &vkbench.CrosConfig{Hasty: true}},
			ExtraAttr: []string{"group:mainline", "informational"},
			Fixture:   "graphicsNoChrome",
			Timeout:   5 * time.Minute,
		}},
	})
}

// VKBench benchmarks the vulkan performance.
func VKBench(ctx context.Context, s *testing.State) {
	config := s.Param().(vkConfig).config
	if err := vkbench.Run(ctx, s.OutDir(), s.FixtValue(), config); err != nil {
		s.Fatal("Failed to run vkbench: ", err)
	}
}
