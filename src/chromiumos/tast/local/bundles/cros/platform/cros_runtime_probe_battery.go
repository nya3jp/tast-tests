// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/godbus/dbus"

	rppb "chromiumos/system_api/runtime_probe_proto"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosRuntimeProbeBattery,
		Desc: "Checks that battery probe results are expected",
		Contacts: []string{
			"ckclark@chromium.org",
			"chromeos-runtime-probe@google.com",
		},
		Attr:         []string{"group:runtime_probe"},
		SoftwareDeps: []string{"wilco"},
		Vars:         []string{"autotest.host_info_labels"},
	})
}

// getBatteryNamesFromCrosLabels will extract the model and battery names (prefixed with
// "hwid_component:battery/") from autotest.host_info_labels var which is a json string of list of cros-labels.
// After collecting battery names, this function will return a set containing them.
func getBatteryNamesFromCrosLabels(s *testing.State) (map[string]struct{}, bool) {
	var labelsStr string
	var ok bool
	if labelsStr, ok = s.Var("autotest.host_info_labels"); !ok {
		s.Log("No info labels, skipped")
		return nil, false
	}
	var labels []string
	if err := json.Unmarshal([]byte(labelsStr), &labels); err != nil {
		s.Fatal("Unabled to decode autotest.host_info_labels: ", err)
	}
	// Filter labels with prefix "hwid_component:battery/" and trim them
	var batteryNames []string
	var model string
	for _, label := range labels {
		if strings.HasPrefix(label, "hwid_component:battery/") {
			batteryLabel := strings.TrimPrefix(label, "hwid_component:battery/")
			batteryNames = append(batteryNames, batteryLabel)
		} else if strings.HasPrefix(label, "model:") {
			model = strings.TrimPrefix(label, "model:")
		}
	}
	batterySet := make(map[string]struct{})
	for _, batteryLabel := range batteryNames {
		batterySet[model+"_"+batteryLabel] = struct{}{}
	}

	return batterySet, true
}

// getProbedBatteryComponents uses D-Bus call to get battery components from runtime_probe
func getProbedBatteryComponents(ctx context.Context, s *testing.State) []*rppb.Battery {
	const (
		// Define the D-Bus constants here.
		// Note that this is for the reference only to demonstrate how
		// to use dbusutil. For actual use, session_manager D-Bus call
		// should be performed via
		// chromiumos/tast/local/session_manager pacakge.
		jobName       = "runtime_probe"
		dbusName      = "org.chromium.RuntimeProbe"
		dbusPath      = "/org/chromium/RuntimeProbe"
		dbusInterface = "org.chromium.RuntimeProbe"
		dbusMethod    = dbusInterface + ".ProbeCategories"
	)

	if err := upstart.EnsureJobRunning(ctx, jobName); err != nil {
		s.Fatal("Runtime probe is not running: ", err)
	}

	_, obj, err := dbusutil.Connect(ctx, dbusName, dbus.ObjectPath(dbusPath))
	if err != nil {
		s.Fatalf("Failed to connect to %s: %v", dbusName, err)
	}

	request := rppb.ProbeRequest{
		Categories: []rppb.ProbeRequest_SupportCategory{
			rppb.ProbeRequest_battery,
		},
	}
	result := rppb.ProbeResult{}

	if err := dbusutil.CallProtoMethod(ctx, obj, dbusMethod, &request, &result); err != nil {
		s.Fatalf("Failed to call method %s: %v", dbusMethod, err)
	}
	return result.GetBattery()
}

// CrosRuntimeProbeBattery checks if the battery names in cros-label are consistent with probed names from runtime_probe
func CrosRuntimeProbeBattery(ctx context.Context, s *testing.State) {
	var batterySet map[string]struct{}
	var ok bool

	if batterySet, ok = getBatteryNamesFromCrosLabels(s); !ok || len(batterySet) == 0 {
		// TODO: Fail this case after all tests will set cros-labels
		s.Log("No battery labels")
		return
	}

	var probedBatteryComponents = getProbedBatteryComponents(ctx, s)
	for _, component := range probedBatteryComponents {
		name := component.GetName()
		if name == "generic" {
			s.Log("Skip known generic probe result")
		} else {
			s.Log("Probed battery:", name)
			if _, exists := batterySet[name]; !exists {
				s.Fatalf("Unexpected battery %v is probed", name)
			}
			delete(batterySet, name)
		}
	}
	if len(batterySet) > 0 {
		unprobedBatteries := make([]string, 0, len(batterySet))
		for k := range batterySet {
			unprobedBatteries = append(unprobedBatteries, k)
		}
		s.Fatal("Some batteries are not probed:", unprobedBatteries)
	}
}
