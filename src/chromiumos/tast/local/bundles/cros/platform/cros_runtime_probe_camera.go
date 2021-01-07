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
		Func: CrosRuntimeProbeCamera,
		Desc: "Checks that camera probe results are expected",
		Contacts: []string{
			"ckclark@chromium.org",
			"chromeos-runtime-probe@google.com",
		},
		Attr:         []string{"group:runtime_probe"},
		SoftwareDeps: []string{"racc"},
		Vars:         []string{"autotest_host_info_labels"},
	})
}

// CrosRuntimeProbeCamera checks if the camera component names in cros-label
// are consistent with probed names from runtime_probe.
func CrosRuntimeProbeCamera(ctx context.Context, s *testing.State) {
	categories := []string{"video", "camera"}
	labelsStr, ok := s.Var("autotest_host_info_labels")
	if !ok {
		s.Fatal("No camera labels")
	}

	mapping, model, err := runtimeprobe.GetComponentCount(labelsStr, categories)
	if err != nil {
		s.Fatal("Unable to decode autotest_host_info_labels: ", err)
	}

	var labels map[string]int
	var category string
	for _, c := range categories {
		if len(mapping[c]) > 0 {
			labels, category = mapping[c], c
			// We will not have to look the other category since we are expecting
			// only one of them present in DUT Labels.
			break
		}
	}

	if labels == nil {
		s.Fatal("No camera labels")
	}

	request := &rppb.ProbeRequest{
		Categories: []rppb.ProbeRequest_SupportCategory{
			rppb.ProbeRequest_camera,
		},
	}
	result, err := runtimeprobe.Probe(ctx, request)
	if err != nil {
		s.Fatal("Cannot get camera components: ", err)
	}
	probedCameraComponents := result.GetCamera()

	for _, component := range probedCameraComponents {
		result, name := runtimeprobe.DecreaseComponentCount(labels, model, component)
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
		unprobedCameras := make([]string, 0, len(labels))
		for k := range labels {
			unprobedCameras = append(unprobedCameras, k)
		}
		sort.Strings(unprobedCameras)
		s.Fatal("Some camera(s) are not probed: ", unprobedCameras)
	}
}
