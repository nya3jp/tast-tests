// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package runtimeprobe provides utilities for runtime_probe tests.
package runtimeprobe

import (
	"context"
	"encoding/json"
	"regexp"
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

// Probe uses D-Bus call to get result from runtime_probe with given request.
// Currently only users chronos and debugd are allowed to call this D-Bus function.
func Probe(ctx context.Context, request *rppb.ProbeRequest) (*rppb.ProbeResult, error) {
	result := &rppb.ProbeResult{}
	err := dbusCallWithRetry(ctx, "ProbeCategories", request, result, defaultTryCount)
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

// GetKnownComponents uses D-Bus call to get known components with category
// |category|.
func GetKnownComponents(ctx context.Context, model, category string, tryCount int) (map[string]struct{}, error) {
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
		knownComponents, err := GetKnownComponents(ctx, model, category, defaultTryCount)
		if err != nil {
			return nil, "", err
		}
		for _, alias := range categoryAliases(category) {
			categoryPrefix := "hwid_component:" + alias + "/"
			for _, label := range labels {
				if !strings.HasPrefix(label, categoryPrefix) {
					continue
				}
				label := strings.TrimPrefix(label, categoryPrefix)
				key := tryTrimQid(model, category, model+"_"+label)
				if _, found := knownComponents[key]; found {
					mapping[category][key]++
				}
			}
		}
	}
	return mapping, model, nil
}

// DecreaseComponentCount decreases the count of given component by 1.  If the
// count of given component is decreased to 0, it will be removed from |count|.
// The first returned value will be false on failure.  The second returned
// value is the display name of |component|.
func DecreaseComponentCount(count map[string]int, model, category string, component Component) (bool, string) {
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
