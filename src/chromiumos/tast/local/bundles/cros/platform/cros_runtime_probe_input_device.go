// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"

	rppb "chromiumos/system_api/runtime_probe_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/platform/runtimeprobe"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosRuntimeProbeInputDevice,
		Desc: "Checks that input_device probe results are expected",
		Contacts: []string{
			"ckclark@chromium.org",
			"chromeos-runtime-probe@google.com",
		},
		Attr:         []string{"group:runtime_probe"},
		SoftwareDeps: []string{"racc"},
		Vars:         []string{"autotest_host_info_labels"},
	})
}

// CrosRuntimeProbeInputDevice checks if the input_device names in cros-label
// are consistent with probed names from runtime_probe.
func CrosRuntimeProbeInputDevice(ctx context.Context, s *testing.State) {
	categories := []string{"stylus", "touchpad", "touchscreen"}
	getCategoryComps := func(result *rppb.ProbeResult, category string) ([]runtimeprobe.Component, error) {
		var comps []runtimeprobe.Component
		var rppbComps []*rppb.InputDevice
		switch category {
		case "stylus":
			rppbComps = result.GetStylus()
		case "touchpad":
			rppbComps = result.GetTouchpad()
		case "touchscreen":
			rppbComps = result.GetTouchscreen()
		default:
			return nil, errors.Errorf("unknown category %s", category)
		}
		for _, comp := range rppbComps {
			comps = append(comps, comp)
		}
		return comps, nil
	}
	runtimeprobe.GenericTest(ctx, s, categories, getCategoryComps, true)
}
