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

var memoryTypes = []string{"dram"}

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

// memoryNameMapping will extract the model and memory names (prefixed with
// "hwid_component:<memory type>/") from autotest_host_info_labels var which
// is a json string of list of cros-labels.  After collecting memory names,
// this function will return a map of set containing them by memory type.
// Since we need the model name for component group, here we return it as well.
func memoryNameMapping(jsonStr string) (map[string]map[string]struct{}, string, error) {
	const modelPrefix = "model:"
	mapping := make(map[string]map[string]struct{})
	for _, memoryType := range memoryTypes {
		mapping[memoryType] = make(map[string]struct{})
	}

	var labels []string
	if err := json.Unmarshal([]byte(jsonStr), &labels); err != nil {
		return nil, "", err
	}
	// Find the model name of this DUT.
	var model string
	for _, label := range labels {
		if strings.HasPrefix(label, modelPrefix) {
			model = strings.TrimPrefix(label, modelPrefix)
			break
		}
	}
	if len(model) == 0 {
		return nil, "", errors.New("no model found")
	}

	// Filter labels with prefix "hwid_component:<memory type>/" and trim them.
	for _, label := range labels {
		for _, memoryType := range memoryTypes {
			memoryPrefix := "hwid_component:" + memoryType + "/"
			if strings.HasPrefix(label, memoryPrefix) {
				label := strings.TrimPrefix(label, memoryPrefix)
				mapping[memoryType][model+"_"+label] = struct{}{}
				break
			}
		}
	}

	return mapping, model, nil
}

func getMemoryByType(result *rppb.ProbeResult, memoryType string) ([]*rppb.Memory, error) {
	switch memoryType {
	case "dram":
		return result.GetDram(), nil
	}
	return nil, errors.Errorf("unknown device_type %s", memoryType)
}

// CrosRuntimeProbeMemory checks if the memory names in cros-label
// are consistent with probed names from runtime_probe.
func CrosRuntimeProbeMemory(ctx context.Context, s *testing.State) {
	labelsStr, ok := s.Var("autotest_host_info_labels")
	if !ok {
		s.Fatal("No memory labels")
	}

	mapping, model, err := memoryNameMapping(labelsStr)
	if err != nil {
		s.Fatal("Unable to decode autotest_host_info_labels: ", err)
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

	for _, memoryType := range memoryTypes {
		probedMemoryComponents, err := getMemoryByType(result, memoryType)
		if err != nil {
			s.Fatal("Cannot get memory: ", err)
		}
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
				if _, exists := mapping[memoryType][name]; !exists {
					s.Logf("Extra memory component %v of type %v is probed", name, memoryType)
				} else {
					delete(mapping[memoryType], name)
				}
			}
		}
	}
	var unprobedMemorys []string
	for memoryType, memoryNames := range mapping {
		for name := range memoryNames {
			unprobedMemorys = append(unprobedMemorys, memoryType+"/"+name)
		}
	}
	if len(unprobedMemorys) > 0 {
		sort.Strings(unprobedMemorys)
		s.Fatal("Some expected memory components are not probed: ", unprobedMemorys)
	}
}
