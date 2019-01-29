// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"fmt"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         USBGuard,
		Desc:         "Checks iptables and ip6tables firewall rules",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"usbguard"},
	})
}

const (
	usbguardFeature   = "USBGuard"
	usbbouncerFeature = "USBBouncer"
)

func boolToEnable(value bool) string {
	if value {
		return "enable"
	}
	return "disable"
}

func isFeatureEnabled(ctx context.Context, s *testing.State, feature string) (bool, error) {
	const (
		dbusName      = "org.chromium.ChromeFeaturesService"
		dbusPath      = "/org/chromium/ChromeFeaturesService"
		dbusInterface = "org.chromium.ChromeFeaturesServiceInterface"
		dbusMethod    = ".IsFeatureEnabled"
	)

	_, obj, err := dbusutil.Connect(ctx, dbusName, dbus.ObjectPath(dbusPath))
	if err != nil {
		s.Fatalf("Failed to connect to %s: %v", dbusName, err)
	}

	s.Logf("Asking chrome is %q is enabled", feature)
	state := false
	if err := obj.CallWithContext(ctx, dbusInterface+dbusMethod, 0, feature).Store(&state); err != nil {
		s.Error("Failed to get session state: ", err)
	} else {
		s.Logf("IsFeatureEnabled(%q) -> %t", feature, state)
	}
	return state, err
}

func checkEnabled(ctx context.Context, s *testing.State, feature string, expected bool) error {
	enabled, err := isFeatureEnabled(ctx, s, feature)
	if err != nil {
		return err
	}
	if enabled != expected {
		msg := fmt.Sprintf("Got unexpected feature state of %q! (%t != %t)", feature, expected, enabled)
		s.Error(msg)
		return errors.New(msg)
	}
	return nil
}

func singleUSBGuardTest(ctx context.Context, s *testing.State, usbguardEnabled bool, usbbouncerEnabled bool) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs([]string{
		"--" + boolToEnable(usbguardEnabled) + "-features=" + usbguardFeature,
		"--" + boolToEnable(usbbouncerEnabled) + "-features=" + usbbouncerFeature}))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	if checkEnabled(ctx, s, usbguardFeature, usbguardEnabled) != nil ||
		checkEnabled(ctx, s, usbbouncerFeature, usbbouncerEnabled) != nil {
		return
	}

	// TODO(allenwebb) emit upstart events and check usbguard state
}

func USBGuard(ctx context.Context, s *testing.State) {
	singleUSBGuardTest(ctx, s, true, true)
	singleUSBGuardTest(ctx, s, true, false)
	singleUSBGuardTest(ctx, s, false, true)
	singleUSBGuardTest(ctx, s, false, false)
}
