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
		Func: CrosRuntimeProbeCamera,
		Desc: "Checks that camera probe results are expected",
		Contacts: []string{
			"ckclark@chromium.org",
			"chromeos-runtime-probe@google.com",
		},
		Attr:         []string{"group:runtime_probe"},
		SoftwareDeps: []string{"wilco"},
		Vars:         []string{"autotest_host_info_labels"},
	})
}

// cameraNames will extract the model and camera names (prefixed with
// "hwid_component:video/") from autotest_host_info_labels var which is a json
// string of list of cros-labels.  After collecting camera names, this function
// will return a counter counting each name.  Since we need the model name for
// component group, here we return it as well.
func cameraNames(jsonStr string) (map[string]int, string, error) {
	const (
		memoryPrefix = "hwid_component:video/"
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

	count := make(map[string]int)
	for _, label := range names {
		key := model + "_" + label
		if _, ok := count[key]; ok {
			count[key]++
		} else {
			count[key] = 1
		}
	}

	return count, model, nil
}

// CrosRuntimeProbeCamera checks if the camera component names in cros-label
// are consistent with probed names from runtime_probe.
func CrosRuntimeProbeCamera(ctx context.Context, s *testing.State) {
	labelsStr, ok := s.Var("autotest_host_info_labels")
	if !ok {
		s.Fatal("No camera labels")
	}

	count, model, err := cameraNames(labelsStr)
	if err != nil {
		s.Fatal("Unable to decode autotest_host_info_labels: ", err)
	} else if len(count) == 0 {
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
		name := component.GetName()
		if info := component.GetInformation(); info != nil {
			if compGroup := info.GetCompGroup(); compGroup != "" {
				name = model + "_" + compGroup
			}
		}
		if name == "generic" {
			s.Log("Skip known generic probe result")
		} else {
			s.Log("Probed camera: ", name)
			if _, exists := count[name]; !exists {
				s.Fatalf("Unexpected camera %v is probed", name)
			}
			count[name]--
			if count[name] == 0 {
				delete(count, name)
			}
		}
	}

	if len(count) > 0 {
		unprobedCameras := make([]string, 0, len(count))
		for k := range count {
			unprobedCameras = append(unprobedCameras, k)
		}
		sort.Strings(unprobedCameras)
		s.Fatal("Some camera(s) are not probed: ", unprobedCameras)
	}
}
