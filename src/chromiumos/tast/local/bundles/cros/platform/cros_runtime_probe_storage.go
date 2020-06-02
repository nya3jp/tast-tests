// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
		Func: CrosRuntimeProbeStorage,
		Desc: "Checks that storage probe results are expected",
		Contacts: []string{
			"ckclark@chromium.org",
			"chromeos-runtime-probe@google.com",
		},
		Attr:         []string{"group:runtime_probe"},
		SoftwareDeps: []string{"wilco"},
		Vars:         []string{"autotest_host_info_labels"},
	})
}

// storageNames will extract the model and storage names (prefixed with
// "hwid_component:storage/") from autotest_host_info_labels var which is a json string of list of cros-labels.
// After collecting storage names, this function will return a set containing them.
// Since we need the model name for component group, here we return it as well.
func storageNames(jsonStr string) (map[string]struct{}, string, error) {
	const (
		storagePrefix = "hwid_component:storage/"
		modelPrefix   = "model:"
	)
	var labels []string
	if err := json.Unmarshal([]byte(jsonStr), &labels); err != nil {
		return nil, "", err
	}
	// Filter labels with prefix "hwid_component:storage/" and trim them.
	// Also find the model name of this DUT.
	var names []string
	var model string
	for _, label := range labels {
		if strings.HasPrefix(label, storagePrefix) {
			label := strings.TrimPrefix(label, storagePrefix)
			names = append(names, label)
		} else if strings.HasPrefix(label, modelPrefix) {
			model = strings.TrimPrefix(label, modelPrefix)
		}
	}
	if len(model) == 0 {
		return nil, "", errors.New("no model found")
	}

	set := make(map[string]struct{})
	for _, label := range names {
		set[model+"_"+label] = struct{}{}
	}

	return set, model, nil
}

// CrosRuntimeProbeStorage checks if the storage names in cros-label are consistent with probed names from runtime_probe
func CrosRuntimeProbeStorage(ctx context.Context, s *testing.State) {
	labelsStr, ok := s.Var("autotest_host_info_labels")
	if !ok {
		s.Fatal("No storage labels")
	}

	set, model, err := storageNames(labelsStr)
	if err != nil {
		s.Fatal("Unable to decode autotest_host_info_labels: ", err)
	}

	if len(set) == 0 {
		s.Fatal("No storage labels")
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
		name := component.GetName()
		if info := component.GetInformation(); info != nil {
			if compGroup := info.GetCompGroup(); compGroup != "" {
				name = model + "_" + compGroup
			}
		}
		if name == "generic" {
			s.Log("Skip known generic probe result")
		} else {
			s.Log("Probed storage:", name)
			if _, exists := set[name]; !exists {
				s.Fatalf("Unexpected storage %v is probed", name)
			}
			delete(set, name)
		}
	}
	if len(set) > 0 {
		unprobedStorages := make([]string, 0, len(set))
		for k := range set {
			unprobedStorages = append(unprobedStorages, k)
		}
		sort.Strings(unprobedStorages)
		s.Fatal("Some storages are not probed: ", unprobedStorages)
	}
}
