// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"sort"

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
	categories := []rppb.ProbeRequest_SupportCategory{
		rppb.ProbeRequest_cellular,
		rppb.ProbeRequest_ethernet,
		rppb.ProbeRequest_wireless,
	}
	var networkTypes = make([]string, len(categories))
	for i, category := range categories {
		networkTypes[i] = category.String()
	}
	hostInfoLabels, err := runtimeprobe.GetHostInfoLabels(s)
	if err != nil {
		s.Fatal("GetHostInfoLabels failed: ", err)
	}

	mapping, model, err := runtimeprobe.GetComponentCount(ctx, hostInfoLabels, networkTypes)
	if err != nil {
		s.Fatal("Unable to decode autotest_host_info_labels: ", err)
	}

	request := &rppb.ProbeRequest{
		Categories: categories,
	}
	result, err := runtimeprobe.Probe(ctx, request)
	if err != nil {
		s.Fatal("Cannot get network components: ", err)
	}

	getNetworkByType := func(result *rppb.ProbeResult, networkType string) ([]*rppb.Network, error) {
		switch networkType {
		case "cellular":
			return result.GetCellular(), nil
		case "ethernet":
			return result.GetEthernet(), nil
		case "wireless":
			return result.GetWireless(), nil
		}
		return nil, errors.Errorf("unknown device_type %s", networkType)
	}

	for _, networkType := range networkTypes {
		probedNetworkComponents, err := getNetworkByType(result, networkType)
		if err != nil {
			s.Error("Cannot get network: ", err)
			continue
		}
		for _, component := range probedNetworkComponents {
			result, name := runtimeprobe.DecreaseComponentCount(mapping[networkType], model, component)
			s.Logf("Probed %s: %s", networkType, name)
			if !result {
				if name == "generic" {
					s.Logf("Skip known generic %s probe result", networkType)
				} else {
					s.Logf("Extra network component %q of type %s is probed", name, networkType)
				}
			}
		}
	}
	var unprobedNetworks []string
	for networkType, networkNames := range mapping {
		for name := range networkNames {
			unprobedNetworks = append(unprobedNetworks, networkType+"/"+name)
		}
	}
	if len(unprobedNetworks) > 0 {
		sort.Strings(unprobedNetworks)
		s.Fatal("Some expected network components are not probed: ", unprobedNetworks)
	}
}
