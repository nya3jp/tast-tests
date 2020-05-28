// Copyright 2020 The Chromium OS Authors. All rights reserved.
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

var networkTypes = []string{"wireless", "cellular", "ethernet"}

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosRuntimeProbeNetwork,
		Desc: "Checks that network probe results are expected",
		Contacts: []string{
			"ckclark@chromium.org",
			"chromeos-runtime-probe@google.com",
		},
		Attr:         []string{"group:runtime_probe"},
		SoftwareDeps: []string{"wilco"},
		Vars:         []string{"autotest_host_info_labels"},
	})
}

// networkNameMapping will extract the model and network names (prefixed with
// "hwid_component:<network type>/") from autotest_host_info_labels var which
// is a json string of list of cros-labels.  After collecting network names,
// this function will return a map of set containing them by network type.
// Since we need the model name for component group, here we return it as well.
func networkNameMapping(jsonStr string) (map[string]map[string]struct{}, string, error) {
	const modelPrefix = "model:"
	mapping := make(map[string]map[string]struct{})
	for _, networkType := range networkTypes {
		mapping[networkType] = make(map[string]struct{})
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

	// Filter labels with prefix "hwid_component:<network type>/" and trim them.
	for _, label := range labels {
		for _, networkType := range networkTypes {
			networkPrefix := "hwid_component:" + networkType + "/"
			if strings.HasPrefix(label, networkPrefix) {
				label := strings.TrimPrefix(label, networkPrefix)
				mapping[networkType][model+"_"+label] = struct{}{}
				break
			}
		}
	}

	return mapping, model, nil
}

// networkComponents uses D-Bus call to get network components from runtime_probe.
// Currently only users |chronos| and |debugd| are allowed to call this D-Bus function.
func networkComponents(ctx context.Context) ([]*rppb.Network, error) {
	const (
		// Define the D-Bus constants here.
		// Note that this is for the reference only to demonstrate how
		// to use dbusutil. For actual use, session_manager D-Bus call
		// should be performed via
		// chromiumos/tast/local/session_manager package.
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
			rppb.ProbeRequest_network,
		},
	}
	result := rppb.ProbeResult{}

	if err := dbusutil.CallProtoMethod(ctx, obj, dbusMethod, &request, &result); err != nil {
		return nil, errors.Wrapf(err, "failed to call method %s", dbusMethod)
	}
	return result.GetNetwork(), nil
}

// CrosRuntimeProbeNetwork checks if the network names in cros-label are
// consistent with probed names from runtime_probe.
func CrosRuntimeProbeNetwork(ctx context.Context, s *testing.State) {
	labelsStr, ok := s.Var("autotest_host_info_labels")
	if !ok {
		s.Fatal("No network labels")
	}

	mapping, model, err := networkNameMapping(labelsStr)
	if err != nil {
		s.Fatal("Unable to decode autotest_host_info_labels: ", err)
	}

	probedNetworkComponents, err := networkComponents(ctx)
	if err != nil {
		s.Fatal("Cannot get network components: ", err)
	}

	for _, component := range probedNetworkComponents {
		name := component.GetName()
		if info := component.GetInformation(); info != nil {
			if compGroup := info.GetCompGroup(); compGroup != "" {
				name = model + "_" + compGroup
			}
		}
		values := component.GetValues()
		networkType := values.GetType()
		if name == "generic" {
			s.Log("Skip known generic probe result")
		} else {
			s.Log("Probed network:", name)
			if _, exists := mapping[networkType][name]; !exists {
				s.Logf("Extra network component %v of type %v is probed", name, networkType)
			} else {
				delete(mapping[networkType], name)
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
