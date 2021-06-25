// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"sort"

	rppb "chromiumos/system_api/runtime_probe_proto"
	"chromiumos/tast/local/bundles/cros/platform/runtimeprobe"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosRuntimeProbeEdid,
		Desc: "Checks that edid probe results are expected",
		Contacts: []string{
			"ckclark@chromium.org",
			"chromeos-runtime-probe@google.com",
		},
		Attr:         []string{"group:runtime_probe"},
		SoftwareDeps: []string{"racc"},
		Vars:         []string{"autotest_host_info_labels"},
	})
}

// CrosRuntimeProbeEdid checks if the edid names in cros-label are
// consistent with probed names from runtime_probe.
func CrosRuntimeProbeEdid(ctx context.Context, s *testing.State) {
	const category = "display_panel"
	hostInfoLabels, err := runtimeprobe.GetHostInfoLabels(s)
	if err != nil {
		s.Fatal("GetHostInfoLabels failed: ", err)
	}

	mapping, model, err := runtimeprobe.GetComponentCount(ctx, hostInfoLabels, []string{category})
	if err != nil {
		s.Fatal("Unable to decode autotest_host_info_labels: ", err)
	}
	labels := mapping[category]
	if len(labels) == 0 {
		s.Log("No edid labels or known components. Skipped")
		return
	}

	request := &rppb.ProbeRequest{
		Categories: []rppb.ProbeRequest_SupportCategory{
			rppb.ProbeRequest_display_panel,
		},
	}
	result, err := runtimeprobe.Probe(ctx, request)
	if err != nil {
		s.Fatal("Cannot get edid components: ", err)
	}
	probedEdidComponents := result.GetDisplayPanel()

	for _, component := range probedEdidComponents {
		result, name := runtimeprobe.DecreaseComponentCount(labels, model, category, component)
		s.Logf("Probed %s: %s", category, name)
		if !result {
			if name == "generic" {
				s.Logf("Skip known generic %s probe result", category)
			} else {
				s.Fatalf("Unexpected %s %q is probed", category, name)
			}
		}
	}

	if len(labels) > 0 {
		unprobedEdid := make([]string, 0, len(labels))
		for k := range labels {
			unprobedEdid = append(unprobedEdid, k)
		}
		sort.Strings(unprobedEdid)
		s.Fatal("Some edid(s) are not probed: ", unprobedEdid)
	}
}
