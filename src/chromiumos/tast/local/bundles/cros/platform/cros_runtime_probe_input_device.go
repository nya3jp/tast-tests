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

var inputDeviceTypes = []string{"stylus", "touchpad", "touchscreen"}

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosRuntimeProbeInputDevice,
		Desc: "Checks that input_device probe results are expected",
		Contacts: []string{
			"ckclark@chromium.org",
			"chromeos-runtime-probe@google.com",
		},
		Attr:         []string{"group:runtime_probe"},
		SoftwareDeps: []string{"wilco"},
		Vars:         []string{"autotest_host_info_labels"},
	})
}

// inputDeviceNameMapping will extract the model and input_device names (prefixed with
// "hwid_component:<input_device type>/") from autotest_host_info_labels var which
// is a json string of list of cros-labels.  After collecting input_device names,
// this function will return a map of set containing them by input_device type.
// Since we need the model name for component group, here we return it as well.
func inputDeviceNameMapping(jsonStr string) (map[string]map[string]struct{}, string, error) {
	const modelPrefix = "model:"
	mapping := make(map[string]map[string]struct{})
	for _, inputDeviceType := range inputDeviceTypes {
		mapping[inputDeviceType] = make(map[string]struct{})
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

	// Filter labels with prefix "hwid_component:<input_device type>/" and trim them.
	for _, label := range labels {
		for _, inputDeviceType := range inputDeviceTypes {
			inputDevicePrefix := "hwid_component:" + inputDeviceType + "/"
			if strings.HasPrefix(label, inputDevicePrefix) {
				label := strings.TrimPrefix(label, inputDevicePrefix)
				mapping[inputDeviceType][model+"_"+label] = struct{}{}
				break
			}
		}
	}

	return mapping, model, nil
}

func getInputDeviceByType(result *rppb.ProbeResult, inputDeviceType string) ([]*rppb.InputDevice, error) {
	switch inputDeviceType {
	case "stylus":
		return result.GetStylus(), nil
	case "touchpad":
		return result.GetTouchpad(), nil
	case "touchscreen":
		return result.GetTouchscreen(), nil
	}
	return nil, errors.Errorf("unknown device_type %s", inputDeviceType)
}

// CrosRuntimeProbeInputDevice checks if the input_device names in cros-label
// are consistent with probed names from runtime_probe.
func CrosRuntimeProbeInputDevice(ctx context.Context, s *testing.State) {
	labelsStr, ok := s.Var("autotest_host_info_labels")
	if !ok {
		s.Fatal("No input_device labels")
	}

	mapping, model, err := inputDeviceNameMapping(labelsStr)
	if err != nil {
		s.Fatal("Unable to decode autotest_host_info_labels: ", err)
	}

	request := &rppb.ProbeRequest{
		Categories: []rppb.ProbeRequest_SupportCategory{
			rppb.ProbeRequest_stylus,
			rppb.ProbeRequest_touchpad,
			rppb.ProbeRequest_touchscreen,
		},
	}
	result, err := runtimeprobe.Probe(ctx, request)
	if err != nil {
		s.Fatal("Cannot get input_device components: ", err)
	}

	for _, inputDeviceType := range inputDeviceTypes {
		probedInputDeviceComponents, err := getInputDeviceByType(result, inputDeviceType)
		if err != nil {
			s.Fatal("Cannot get input_device: ", err)
		}
		for _, component := range probedInputDeviceComponents {
			name := component.GetName()
			if info := component.GetInformation(); info != nil {
				if compGroup := info.GetCompGroup(); compGroup != "" {
					name = model + "_" + compGroup
				}
			}
			if name == "generic" {
				s.Log("Skip known generic probe result")
			} else {
				s.Log("Probed input_device: ", name)
				if _, exists := mapping[inputDeviceType][name]; !exists {
					s.Logf("Extra inputDevice component %v of type %v is probed", name, inputDeviceType)
				} else {
					delete(mapping[inputDeviceType], name)
				}
			}
		}
	}
	var unprobedInputDevices []string
	for inputDeviceType, inputDeviceNames := range mapping {
		for name := range inputDeviceNames {
			unprobedInputDevices = append(unprobedInputDevices, inputDeviceType+"/"+name)
		}
	}
	if len(unprobedInputDevices) > 0 {
		sort.Strings(unprobedInputDevices)
		s.Fatal("Some expected input_device components are not probed: ", unprobedInputDevices)
	}
}
