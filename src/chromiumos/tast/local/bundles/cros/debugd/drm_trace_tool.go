// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package debugd

import (
	"context"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

const (
	dbusName      = "org.chromium.debugd"
	dbusPath      = "/org/chromium/debugd"
	dbusInterface = "org.chromium.debugd"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DrmTraceTool,
		Desc: "Tests D-bus methods related to DrmTraceTool",
		Contacts: []string{
			"ddavenport@chromium.org",
		},
		Attr: []string{"group:mainline", "informational"},
		// TODO: Limit to run only on newer kernel versions
	})
}

// DrmTraceTool tests D-bus methods related to debugd's DrmTraceTool.
func DrmTraceTool(ctx context.Context, s *testing.State) {
	_, obj, err := dbusutil.Connect(ctx, dbusName, dbus.ObjectPath(dbusPath))
	if err != nil {
		s.Fatalf("Failed to connect to %s: %v", dbusName, err)
	}

	s.Log("Verify DrmTraceAnnotateLog method")
	// Create a log annotation that will not already be in the log by chance, or from a previous run of this test.
	log := fmt.Sprintf("annotate-%s", time.Now().Unix())
	testAnnotate(ctx, s, obj, log)

	s.Log("Verify DrmTraceSetCategories method")
	// TODO: Use flags rather than magic numbers.
	testSetCategories(ctx, s, obj, uint32(5), uint32(5))
	testSetCategories(ctx, s, obj, uint32(0), uint32(262))

	s.Log("Verify DrmTraceSetSize method")
	testSetSize(ctx, s, obj)
}

func testAnnotate(ctx context.Context, s *testing.State, obj dbus.BusObject, log string) {
	if err := obj.CallWithContext(ctx, dbusInterface+".DrmTraceAnnotateLog", 0, log).Err; err != nil {
		s.Error("Failed to annotate log: ", err)
	}

	// Check that this ended up in the trace file.
	bytes, err := ioutil.ReadFile("/sys/kernel/debug/tracing/instances/drm/trace")
	if err != nil {
		s.Error("Failed to read trace log: ", err)
	}

	contents := string(bytes)
	if !strings.Contains(contents, log) {
		s.Errorf("Failed to find log %s in drm trace", log)
	}
}

func testSetCategories(ctx context.Context, s *testing.State, obj dbus.BusObject, category, expected uint32) {
	if err := obj.CallWithContext(ctx, dbusInterface+".DrmTraceSetCategories", 0, category).Err; err != nil {
		s.Error("Failed to set categories: ", err)
	}

	// Check that this ended up in the mask file.
	bytes, err := ioutil.ReadFile("/sys/module/drm/parameters/trace")
	if err != nil {
		s.Error("Failed to read trace mask: ", err)
	}
	traceMask := strings.TrimSpace(string(bytes))
	i, err := strconv.Atoi(traceMask)
	if err != nil {
		s.Errorf("Got unexpected value for trace mask: %s", traceMask)
	}

	if i != int(expected) {
		s.Errorf("Unexpected value for trace mask. Got %d and expeced %d", i, expected)
	}
}

func testSetSize(ctx context.Context, s *testing.State, obj dbus.BusObject) {
	// TODO: Pass D-Bus enums as arguments rather than magic constants.

	// Set to default
	if err := obj.CallWithContext(ctx, dbusInterface+".DrmTraceSetSize", 0, uint32(0)).Err; err != nil {
		s.Error("Failed to set size: ", err)
	}

	// Check the size
	defaultSize, err := getTraceBufferSize(s)
	if err != nil {
		s.Error("Failed to read trace buffer size: ", err)
	}

	// Set to debug
	if err := obj.CallWithContext(ctx, dbusInterface+".DrmTraceSetSize", 0, uint32(1)).Err; err != nil {
		s.Error("Failed to set size: ", err)
	}

	debugSize, err := getTraceBufferSize(s)
	if err != nil {
		s.Error("Failed to read trace buffer size: ", err)
	}

	if debugSize <= defaultSize {
		s.Errorf("Expected debug size (%d) to be greater than default size (%d)", debugSize, defaultSize)
	}

	// Set to default
	if err := obj.CallWithContext(ctx, dbusInterface+".DrmTraceSetSize", 0, uint32(0)).Err; err != nil {
		s.Error("Failed to set size: ", err)
	}
}

func readFileToString(s *testing.State, path string) (string, error) {
	var contents string
	bytes, err := ioutil.ReadFile(path)
	if err == nil {
		contents = strings.TrimSpace(string(bytes))
	} else {
		s.Errorf("Failed to read file %s: %v", path, err)
	}
	return contents, err
}

func getTraceBufferSize(s *testing.State) (int, error) {
	var size int
	sizeStr, err := readFileToString(s, "/sys/kernel/debug/tracing/instances/drm/buffer_size_kb")
	if err == nil {
		size, err = strconv.Atoi(sizeStr)
	}
	return size, err
}
