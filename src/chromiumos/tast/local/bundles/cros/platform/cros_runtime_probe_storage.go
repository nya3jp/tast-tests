// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
		Func: CrosRuntimeProbeStorage,
		Desc: "Checks that storage probe results are expected",
		Contacts: []string{
			"ckclark@chromium.org",
			"chromeos-runtime-probe@google.com",
		},
		Attr:         []string{"group:runtime_probe"},
		SoftwareDeps: []string{"racc"},
		Vars:         []string{"autotest_host_info_labels"},
	})
}

// CrosRuntimeProbeStorage checks if the storage names in cros-label are consistent with probed names from runtime_probe
func CrosRuntimeProbeStorage(ctx context.Context, s *testing.State) {
	const category = "storage"
	hostInfoLabels, err := runtimeprobe.GetHostInfoLabels(s)
	if err != nil {
		s.Fatal("GetHostInfoLabels failed: ", err)
	}

	mapping, model, err := runtimeprobe.GetComponentCount(ctx, hostInfoLabels, []string{category})
	labels := mapping[category]
	if err != nil {
		s.Fatal("Unable to decode autotest_host_info_labels: ", err)
	}

	if len(labels) == 0 {
		s.Log("No storage labels or known components. Skipped")
		return
	}

	request := &rppb.ProbeRequest{
		Categories: []rppb.ProbeRequest_SupportCategory{
			rppb.ProbeRequest_storage,
		},
	}
	result, err := runtimeprobe.Probe(ctx, request)
	if err != nil {
		s.Fatal("Cannot get storage components: ", err)
	}
	probedStorageComponents := result.GetStorage()

	for _, component := range probedStorageComponents {
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
		unprobedStorages := make([]string, 0, len(labels))
		for k := range labels {
			unprobedStorages = append(unprobedStorages, k)
		}
		sort.Strings(unprobedStorages)
		s.Fatal("Some storages are not probed: ", unprobedStorages)
	}
}
