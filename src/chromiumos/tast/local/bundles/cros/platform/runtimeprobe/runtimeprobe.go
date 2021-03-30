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
	"github.com/golang/protobuf/proto"

	rppb "chromiumos/system_api/runtime_probe_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// Component represents runtime_probe component interface.
type Component interface {
	GetName() string
	GetInformation() *rppb.Information
}

// dbusCall invokes runtime_probe methods via D-Bus with given input protobuf
// |in|.  If the method called successfully, |out| will be set to the replied
// message and return without errors.  Otherwise an error will be returned.
func dbusCall(ctx context.Context, method string, in, out proto.Message) error {
	const (
		jobName       = "runtime_probe"
		dbusName      = "org.chromium.RuntimeProbe"
		dbusPath      = "/org/chromium/RuntimeProbe"
		dbusInterface = "org.chromium.RuntimeProbe"
	)
	var dbusMethod = dbusInterface + "." + method

	if err := upstart.EnsureJobRunning(ctx, jobName); err != nil {
		return errors.Wrap(err, "runtime probe is not running")
	}
	defer upstart.StopJob(ctx, jobName)

	conn, obj, err := dbusutil.ConnectPrivateWithAuth(ctx, sysutil.ChronosUID, dbusName, dbus.ObjectPath(dbusPath))
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := dbusutil.CallProtoMethod(ctx, obj, dbusMethod, in, out); err != nil {
		return errors.Wrapf(err, "failed to call method %s", dbusMethod)
	}
	return nil
}

// Probe uses D-Bus call to get result from runtime_probe with given request.
// Currently only users chronos and debugd are allowed to call this D-Bus function.
func Probe(ctx context.Context, request *rppb.ProbeRequest) (*rppb.ProbeResult, error) {
	result := &rppb.ProbeResult{}
	err := dbusCall(ctx, "ProbeCategories", request, result)
	return result, err
}

// GetHostInfoLabels get host info labels for tast tests.  If the tast variable
// is not found or is an invalid JSON string, GetHostInfoLabels returns an
// error.
func GetHostInfoLabels(s *testing.State) ([]string, error) {
	labelsStr, ok := s.Var("autotest_host_info_labels")
	if !ok {
		return nil, errors.New("no labels")
	}

	var labels []string
	if err := json.Unmarshal([]byte(labelsStr), &labels); err != nil {
		return nil, err
	}
	return labels, nil
}

// GetKnownComponents uses D-Bus call to get known components with category
// |category|.
func GetKnownComponents(ctx context.Context, category string) (map[string]struct{}, error) {
	categoryValue, found := rppb.ProbeRequest_SupportCategory_value[category]
	if !found {
		return nil, errors.Errorf("invalid category %q", category)
	}
	request := rppb.GetKnownComponentsRequest{
		Category: rppb.ProbeRequest_SupportCategory(categoryValue),
	}
	result := rppb.GetKnownComponentsResult{}
	err := dbusCall(ctx, "GetKnownComponents", &request, &result)
	if err != nil {
		return nil, err
	}
	var components = make(map[string]struct{})
	for _, name := range result.GetComponentNames() {
		components[name] = struct{}{}
	}
	return components, nil
}

// GetComponentCount extracts the model name and component labels from given
// list of cros-labels and groups them by their categories.  After collecting
// component labels, this function will return a counter counting each component
// label.
func GetComponentCount(ctx context.Context, labels, categories []string) (map[string]map[string]int, string, error) {
	const modelPrefix = "model:"

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
	// Filter labels with prefix "hwid_component:<component type>/" and trim them.
	for _, category := range categories {
		knownComponents, err := GetKnownComponents(ctx, category)
		if err != nil {
			return nil, "", err
		}
		categoryPrefix := "hwid_component:" + category + "/"
		for _, label := range labels {
			if !strings.HasPrefix(label, categoryPrefix) {
				continue
			}
			label := strings.TrimPrefix(label, categoryPrefix)
			key := model + "_" + label
			if _, found := knownComponents[key]; found {
				mapping[category][key]++
			}
		}
	}
	return mapping, model, nil
}

// DecreaseComponentCount decreases the count of given component by 1.  If the
// count of given component is decreased to 0, it will be removed from |count|.
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
