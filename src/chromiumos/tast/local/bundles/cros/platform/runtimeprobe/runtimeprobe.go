// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package runtimeprobe provides utilities for runtime_probe tests.
package runtimeprobe

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
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

// GetComponentsFunc represents the function to get specific category of
// components from rppb.ProbeResult.
type GetComponentsFunc func(*rppb.ProbeResult, string) ([]Component, error)

// Skip the known concurrent D-Bus call at boot.
const defaultTryCount = 2

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

// dbusCallWithRetry wraps dbusCall with retries which try to skip known error
// caused by concurrent D-Bus calls.
func dbusCallWithRetry(ctx context.Context, method string, in, out proto.Message, tryCount int) error {
	var err error
	for i := 0; i < tryCount; i++ {
		if err = dbusCall(ctx, method, in, out); err == nil {
			return nil
		}
	}
	return errors.Wrapf(err, "retry failed for %d times", tryCount)
}

// probe uses D-Bus call to get result from runtime_probe with given request.
// Currently only users chronos and debugd are allowed to call this D-Bus function.
func probe(ctx context.Context, request *rppb.ProbeRequest) (*rppb.ProbeResult, error) {
	result := &rppb.ProbeResult{}
	err := dbusCallWithRetry(ctx, "ProbeCategories", request, result, defaultTryCount)
	return result, err
}

// getHostInfoLabels get host info labels for tast tests.  If the tast variable
// is not found or is an invalid JSON string, this function returns an
// error.
func getHostInfoLabels(s *testing.State) ([]string, error) {
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

// getModelName returns the model name from |labels|.
func getModelName(labels []string) (string, error) {
	const modelPrefix = "model:"

	for _, label := range labels {
		if strings.HasPrefix(label, modelPrefix) {
			model := strings.TrimPrefix(label, modelPrefix)
			return model, nil
		}
	}
	return "", errors.New("no model label found")
}

// categoryAliases returns the aliases of given category.  The category
// "video" is a legacy usage of "camera" category and both follow the
// {category}_{cid}_{qid} name policy.
func categoryAliases(category string) []string {
	if category == "camera" {
		return []string{"camera", "video"}
	}
	return []string{category}
}

// tryTrimQid tries to trim the "_{qid}" suffix in the component name and
// append a fixed string "_{Any}" because tast tests do not care about the
// mutable fields (e.g. firmware) which usually differ in qid but just make
// sure hardware components are probed by probe configs.
func tryTrimQid(model, category, compName string) string {
	aliases := categoryAliases(category)
	aliasPatterns := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		aliasPatterns = append(aliasPatterns, regexp.QuoteMeta(alias))
	}
	pattern := regexp.MustCompile("^(" + regexp.QuoteMeta(model) + "_" + "(?:" + strings.Join(aliasPatterns, "|") + ")" + `_\d+)_\d+(?:#.*)?$`)
	if matches := pattern.FindStringSubmatch(compName); len(matches) > 0 {
		return matches[1] + "_{Any}"
	}
	return compName
}

// getKnownComponents uses D-Bus call to get known components with category
// |category|.
func getKnownComponents(ctx context.Context, model, category string, tryCount int) (map[string]struct{}, error) {
	categoryValue, found := rppb.ProbeRequest_SupportCategory_value[category]
	if !found {
		return nil, errors.Errorf("invalid category %q", category)
	}
	request := rppb.GetKnownComponentsRequest{
		Category: rppb.ProbeRequest_SupportCategory(categoryValue),
	}
	result := rppb.GetKnownComponentsResult{}
	err := dbusCallWithRetry(ctx, "GetKnownComponents", &request, &result, tryCount)
	if err != nil {
		return nil, err
	}
	var components = make(map[string]struct{})
	for _, name := range result.GetComponentNames() {
		trimmedName := tryTrimQid(model, category, name)
		components[trimmedName] = struct{}{}
	}
	return components, nil
}

// getComponentCount returns a counter counting each component label which is
// available on device |model|.
func getComponentCount(labels []string, category, model string, knownComponents map[string]struct{}) map[string]int {
	count := make(map[string]int)
	// Filter labels with prefix "hwid_component:<component type>/" and trim them.
	for _, alias := range categoryAliases(category) {
		categoryPrefix := "hwid_component:" + alias + "/"
		for _, label := range labels {
			if !strings.HasPrefix(label, categoryPrefix) {
				continue
			}
			label := strings.TrimPrefix(label, categoryPrefix)
			key := tryTrimQid(model, category, model+"_"+label)
			if _, found := knownComponents[key]; found {
				count[key]++
			}
		}
	}
	return count
}

// decreaseComponentCount decreases the count of given component by 1.  If the
// count of given component is decreased to 0, it will be removed from |count|.
// The first returned value will be false on failure.  The second returned
// value is the display name of |component|.
func decreaseComponentCount(count map[string]int, model, category string, component Component) (bool, string) {
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
	trimmedName := tryTrimQid(model, category, name)
	if _, exists := count[trimmedName]; !exists {
		return false, name
	}
	count[trimmedName]--
	if count[trimmedName] == 0 {
		delete(count, trimmedName)
	}
	return true, name
}

// GenericTest probes components with category |categories| on a device using
// Runtime Probe D-Bus call and checks if the result matches the host info
// labels.
// If there is not a valid component label, the test will be skipped (passed).
// If a valid component in the host info labels is not probed, the test will be
// failed.  If a probed component is not in the host info labels, the test will
// be failed if |allowExtraComponents| is false.  Otherwire, the test will be
// passed.
func GenericTest(ctx context.Context, s *testing.State, categories []string, getCategoryComps GetComponentsFunc, allowExtraComponents bool) {
	hostInfoLabels, err := getHostInfoLabels(s)
	if err != nil {
		s.Fatal("getHostInfoLabels failed: ", err)
	}

	model, err := getModelName(hostInfoLabels)
	if err != nil {
		s.Fatal("getModelName failed: ", err)
	}

	mapping := make(map[string]map[string]int)
	var requestCategories []rppb.ProbeRequest_SupportCategory
	for _, category := range categories {
		categoryValue, found := rppb.ProbeRequest_SupportCategory_value[category]
		if !found {
			s.Fatalf("Invalid category %q", category)
		}

		knownComponents, err := getKnownComponents(ctx, model, category, defaultTryCount)
		if err != nil {
			s.Fatal("getKnownComponents failed: ", err)
		}
		if len(knownComponents) == 0 {
			s.Logf("Components %q are not found in the probe config. Skipped", category)
			continue
		}

		count := getComponentCount(hostInfoLabels, category, model, knownComponents)
		if len(count) == 0 {
			s.Logf("No %q labels or known components. Skipped", category)
		} else {
			mapping[category] = count
			requestCategories = append(requestCategories, rppb.ProbeRequest_SupportCategory(categoryValue))
		}
	}

	request := &rppb.ProbeRequest{
		Categories: requestCategories,
	}
	result, err := probe(ctx, request)
	if err != nil {
		s.Fatal("probe failed: ", err)
	}

	for category, compCounts := range mapping {
		probedComponents, err := getCategoryComps(result, category)
		var extraComponents []string
		if err != nil {
			s.Error("getCategoryComps failed: ", err)
			continue
		}
		for _, component := range probedComponents {
			result, name := decreaseComponentCount(compCounts, model, category, component)
			s.Logf("Probed %s component: %s", category, name)
			if !result {
				if name != "generic" {
					extraComponents = append(extraComponents, category+"/"+name)
				}
			}
		}

		var unprobedComponents []string
		for name := range compCounts {
			unprobedComponents = append(unprobedComponents, category+"/"+name)
		}
		if len(unprobedComponents) > 0 {
			sort.Strings(unprobedComponents)
			s.Fatalf("Some expected %s components are not probed: %v", category, unprobedComponents)
		}

		if len(extraComponents) > 0 {
			sort.Strings(extraComponents)
			if allowExtraComponents {
				s.Logf("Some extra %s components are probed: %v", category, extraComponents)
			} else {
				s.Fatalf("Some extra %s components are probed: %v", category, extraComponents)
			}
		}
	}
}
