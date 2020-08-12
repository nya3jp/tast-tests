// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package runtimeprobe provides utilities for runtime_probe tests.
package runtimeprobe

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/godbus/dbus"

	rppb "chromiumos/system_api/runtime_probe_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
)

// Component represents runtime_probe component interface.
type Component interface {
	GetName() string
	GetInformation() *rppb.Information
}

// Probe uses D-Bus call to get result from runtime_probe with given request.
// Currently only users chronos and debugd are allowed to call this D-Bus function.
func Probe(ctx context.Context, request *rppb.ProbeRequest) (*rppb.ProbeResult, error) {
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

	result := &rppb.ProbeResult{}
	if err := dbusutil.CallProtoMethod(ctx, obj, dbusMethod, request, result); err != nil {
		return nil, errors.Wrapf(err, "failed to call method %s", dbusMethod)
	}
	return result, nil
}

// GetComponentCount will extract the model name and component labels from given
// json string of list of cros-labels and group them by their categories.  After
// collecting category labels, this function will return a counter counting each
// name.
func GetComponentCount(jsonStr string, categories []string) (map[string]map[string]int, string, error) {
	const modelPrefix = "model:"

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

	mapping := make(map[string]map[string]int)
	for _, category := range categories {
		mapping[category] = make(map[string]int)
	}
	// Filter labels with prefix "hwid_component:<input_device type>/" and trim them.
	for _, label := range labels {
		for _, category := range categories {
			categoryPrefix := "hwid_component:" + category + "/"
			if strings.HasPrefix(label, categoryPrefix) {
				label := strings.TrimPrefix(label, categoryPrefix)
				key := model + "_" + label
				if _, ok := mapping[category][key]; ok {
					mapping[category][key]++
				} else {
					mapping[category][key] = 1
				}
			}
		}
	}
	return mapping, model, nil
}

// DecreaseComponentCount decreases the count of given component by 1.  If the
// count of given component if decreased to 0, it will be removed from |count|.
// The first returned value will be false on failure.  The second returned
// value is the display name of |component|.
func DecreaseComponentCount(count map[string]int, model string, component Component) (bool, string) {
	name := component.GetName()
	info := component.GetInformation()
	if info != nil {
		if compGroup := info.GetCompGroup(); compGroup != "" {
			name = model + "_" + compGroup
		}
	}
	if name == "generic" {
		return false, name
	}
	if _, exists := count[name]; !exists {
		return false, name
	}
	count[name]--
	if count[name] == 0 {
		delete(count, name)
	}
	return true, name
}
