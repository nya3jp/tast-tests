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
func storageNames(jsonStr string) (map[string]struct{}, error) {
	const (
		storagePrefix = "hwid_component:storage/"
		modelPrefix   = "model:"
	)
	var labels []string
	if err := json.Unmarshal([]byte(jsonStr), &labels); err != nil {
		return nil, err
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
	set := make(map[string]struct{})
	for _, label := range names {
		set[model+"_"+label] = struct{}{}
	}

	return set, nil
}

// storageComponents uses D-Bus call to get storage components from runtime_probe.
// Currently only users |chronos| and |debugd| are allowed to call this D-Bus function.
func storageComponents(ctx context.Context) ([]*rppb.Storage, error) {
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

	conn, obj, err := dbusutil.ConnectPrivateWithAuth(ctx, sysutil.ChronosUID, dbusName, dbus.ObjectPath(dbusPath))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	request := rppb.ProbeRequest{
		Categories: []rppb.ProbeRequest_SupportCategory{
			rppb.ProbeRequest_storage,
		},
	}
	result := rppb.ProbeResult{}

	if err := dbusutil.CallProtoMethod(ctx, obj, dbusMethod, &request, &result); err != nil {
		return nil, errors.Wrapf(err, "failed to call method %s", dbusMethod)
	}
	return result.GetStorage(), nil
}

// CrosRuntimeProbeStorage checks if the storage names in cros-label are consistent with probed names from runtime_probe
func CrosRuntimeProbeStorage(ctx context.Context, s *testing.State) {
	labelsStr, ok := s.Var("autotest_host_info_labels")
	if !ok {
		s.Fatal("No storage labels")
	}

	set, err := storageNames(labelsStr)
	if err != nil {
		s.Fatal("Unable to decode autotest_host_info_labels: ", err)
	}

	if len(set) == 0 {
		s.Fatal("No storage labels")
	}

	probedStorageComponents, err := storageComponents(ctx)
	if err != nil {
		s.Fatal("Cannot get storage components: ", err)
	}
	for _, component := range probedStorageComponents {
		name := component.GetName()
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
