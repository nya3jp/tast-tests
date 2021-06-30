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
		Func: CrosRuntimeProbeNetwork,
		Desc: "Checks that network probe results are expected",
		Contacts: []string{
			"ckclark@chromium.org",
			"chromeos-runtime-probe@google.com",
		},
		Attr:         []string{"group:runtime_probe"},
		SoftwareDeps: []string{"racc"},
		Vars:         []string{"autotest_host_info_labels"},
	})
}

// CrosRuntimeProbeNetwork checks if the network names in cros-label are
// consistent with probed names from runtime_probe.
func CrosRuntimeProbeNetwork(ctx context.Context, s *testing.State) {
	categories := []string{"cellular", "ethernet", "wireless"}
	getCategoryComps := func(result *rppb.ProbeResult, category string) ([]runtimeprobe.Component, error) {
		var comps []runtimeprobe.Component
		var rppbComps []*rppb.Network
		switch category {
		case "cellular":
			rppbComps = result.GetCellular()
		case "ethernet":
			rppbComps = result.GetEthernet()
		case "wireless":
			rppbComps = result.GetWireless()
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
