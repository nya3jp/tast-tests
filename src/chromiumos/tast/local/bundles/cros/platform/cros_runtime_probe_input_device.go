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
		Func: CrosRuntimeProbeInputDevice,
		Desc: "Checks that input_device probe results are expected",
		Contacts: []string{
			"ckclark@chromium.org",
			"chromeos-runtime-probe@google.com",
		},
		Attr:         []string{"group:runtime_probe"},
		SoftwareDeps: []string{"racc"},
		Vars:         []string{"autotest_host_info_labels"},
	})
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
	var inputDeviceTypes = []string{"stylus", "touchpad", "touchscreen"}
	labelsStr, ok := s.Var("autotest_host_info_labels")
	if !ok {
		s.Fatal("No input_device labels")
	}

	mapping, model, err := runtimeprobe.GetComponentCount(labelsStr, inputDeviceTypes)
	if err != nil {
		s.Fatal("Unable to decode autotest_host_info_labels: ", err)
	}

	request := &rppb.ProbeRequest{
		Categories: []rppb.SupportCategory{
			rppb.SupportCategory_stylus,
			rppb.SupportCategory_touchpad,
			rppb.SupportCategory_touchscreen,
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
			result, name := runtimeprobe.DecreaseComponentCount(mapping[inputDeviceType], model, component)
			s.Logf("Probed %s: %s", inputDeviceType, name)
			if !result {
				if name == "generic" {
					s.Logf("Skip known generic %s probe result", inputDeviceType)
				} else {
					s.Logf("Extra inputDevice component %q of type %s is probed", name, inputDeviceType)
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
