// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	rppb "chromiumos/system_api/runtime_probe_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/platform/runtimeprobe"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosRuntimeProbeMemory,
		Desc: "Checks that memory probe results are expected",
		Contacts: []string{
			"ckclark@chromium.org",
			"chromeos-runtime-probe@google.com",
		},
		Attr:         []string{"group:runtime_probe"},
		SoftwareDeps: []string{"wilco"},
		Vars:         []string{"autotest_host_info_labels"},
	})
}

// memoryNames will extract the model and memory names (prefixed with
// "hwid_component:dram/") from autotest_host_info_labels var which is a json
// string of list of cros-labels.  After collecting memory names, this function
// will return a set of integers containing the number of times each name
// appears.  Since we need the model name for component group, here we return it
// as well.
func memoryNames(jsonStr string) (map[string]int, string, error) {
	const (
		memoryPrefix = "hwid_component:dram/"
		modelPrefix  = "model:"
	)
	var labels []string
	if err := json.Unmarshal([]byte(jsonStr), &labels); err != nil {
		return nil, "", err
	}
	// Filter labels with prefix and trim them.
	// Also find the model name of this DUT.
	var names []string
	var model string
	for _, label := range labels {
		if strings.HasPrefix(label, memoryPrefix) {
			label := strings.TrimPrefix(label, memoryPrefix)
			names = append(names, label)
		} else if strings.HasPrefix(label, modelPrefix) {
			model = strings.TrimPrefix(label, modelPrefix)
		}
	}
	if len(model) == 0 {
		return nil, "", errors.New("no model found")
	}

	set := make(map[string]int)
	for _, label := range names {
		key := model + "_" + label
		if _, ok := set[key]; ok {
			set[key]++
		} else {
			set[key] = 1
		}
	}

	return set, model, nil
}

// CrosRuntimeProbeMemory checks if the memory names in cros-label are
// consistent with probed names from runtime_probe.
func CrosRuntimeProbeMemory(ctx context.Context, s *testing.State) {
	labelsStr, ok := s.Var("autotest_host_info_labels")
	if !ok {
		s.Fatal("No memory labels")
	}

	set, model, err := memoryNames(labelsStr)
	if err != nil {
		s.Fatal("Unable to decode autotest_host_info_labels: ", err)
	} else if len(set) == 0 {
		s.Fatal("No memory labels")
	}

	request := &rppb.ProbeRequest{
		Categories: []rppb.ProbeRequest_SupportCategory{
			rppb.ProbeRequest_dram,
		},
	}
	result, err := runtimeprobe.Probe(ctx, request)
	if err != nil {
		s.Fatal("Cannot get memory components: ", err)
	}
	probedMemoryComponents := result.GetDram()

	for _, component := range probedMemoryComponents {
		name := component.GetName()
		if info := component.GetInformation(); info != nil {
			if compGroup := info.GetCompGroup(); compGroup != "" {
				name = model + "_" + compGroup
			}
		}
		if name == "generic" {
			s.Log("Skip known generic probe result")
		} else {
			s.Log("Probed memory: ", name)
			if _, exists := set[name]; !exists {
				s.Fatalf("Unexpected memory %v is probed", name)
			}
			set[name]--
			if set[name] == 0 {
				delete(set, name)
			}
		}
	}

	if len(set) > 0 {
		unprobedMemory := make([]string, 0, len(set))
		for k := range set {
			unprobedMemory = append(unprobedMemory, k)
		}
		sort.Strings(unprobedMemory)
		s.Fatal("Some memory(s) are not probed: ", unprobedMemory)
	}
}
