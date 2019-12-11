// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/godbus/dbus"

	rppb "chromiumos/system_api/runtime_probe_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/sysutil"
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

// batteryNames will extract the model and battery names (prefixed with
// "hwid_component:battery/") from autotest.host_info_labels var which is a json string of list of cros-labels.
// After collecting battery names, this function will return a set containing them.
func batteryNames(jsonStr string) (map[string]struct{}, error) {
	const (
		batteryPrefix = "hwid_component:battery/"
		modelPrefix   = "model:"
	)
	var labels []string
	if err := json.Unmarshal([]byte(jsonStr), &labels); err != nil {
		return nil, err
	}
	// Filter labels with prefix "hwid_component:battery/" and trim them.
	// Also find the model name of this DUT.
	var names []string
	var model string
	for _, label := range labels {
		if strings.HasPrefix(label, batteryPrefix) {
			label := strings.TrimPrefix(label, batteryPrefix)
			names = append(names, label)
		} else if strings.HasPrefix(label, modelPrefix) {
			model = strings.TrimPrefix(label, modelPrefix)
		}
	}
	set := make(map[string]struct{})
	for _, label := range names {
		set[model+"_"+label] = struct{}{}
	}

	return set, nil
}

// batteryComponents uses D-Bus call to get battery components from runtime_probe.
// Currently only users |chronos| and |debugd| are allowed to call this D-Bus function.
func batteryComponents(ctx context.Context) ([]*rppb.Battery, error) {
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
		return nil, errors.Wrap(err, "runtime probe is not running")
	}
	defer upstart.StopJob(ctx, jobName)

	conn, obj, err := dbusutil.ConnectWithAuth(ctx, sysutil.ChronosUID, dbusName, dbus.ObjectPath(dbusPath))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	request := rppb.ProbeRequest{
		Categories: []rppb.ProbeRequest_SupportCategory{
			rppb.ProbeRequest_battery,
		},
	}
	result := rppb.ProbeResult{}

	if err := dbusutil.CallProtoMethod(ctx, obj, dbusMethod, &request, &result); err != nil {
		return nil, errors.Wrapf(err, "failed to call method %s", dbusMethod)
	}
	return result.GetBattery(), nil
}

// CrosRuntimeProbeBattery checks if the battery names in cros-label are consistent with probed names from runtime_probe
func CrosRuntimeProbeBattery(ctx context.Context, s *testing.State) {
	labelsStr, ok := s.Var("autotest.host_info_labels")
	if !ok {
		s.Fatal("No battery labels")
	}

	set, err := batteryNames(labelsStr)
	if err != nil {
		s.Fatal("Unable to decode autotest.host_info_labels: ", err)
	}

	if len(set) == 0 {
		s.Fatal("No battery labels")
	}

	probedBatteryComponents, err := batteryComponents(ctx)
	if err != nil {
		s.Fatal("Cannot get battery components: ", err)
	}
	for _, component := range probedBatteryComponents {
		name := component.GetName()
		if name == "generic" {
			s.Log("Skip known generic probe result")
		} else {
			s.Log("Probed battery:", name)
			if _, exists := set[name]; !exists {
				s.Fatalf("Unexpected battery %v is probed", name)
			}
			delete(set, name)
		}
	}
	if len(set) > 0 {
		unprobedBatteries := make([]string, 0, len(set))
		for k := range set {
			unprobedBatteries = append(unprobedBatteries, k)
		}
		sort.Strings(unprobedBatteries)
		s.Fatal("Some batteries are not probed: ", unprobedBatteries)
	}
}
