// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package adb

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/errors"
)

func TestPareDevice(t *testing.T) {
	for _, tc := range []struct {
		line         string
		wantedDevice *Device
		wantedErr    error
	}{
		{"", nil, errSkippedLine},
		{"List of devices attached", nil, errSkippedLine},
		{"* daemon not running. starting it now on port 5037 *", nil, errSkippedLine},
		{"random", nil, errUnexpectedLine},
		{"serial device", &Device{Serial: "serial"}, nil},
		{"serial offline", &Device{Serial: "serial"}, nil},
		{"serial bad-state", nil, errUnexpectedDeviceState},
		{"serial device device:device model:model product:product transport_id:transport_id", &Device{Serial: "serial", Device: "device", Model: "model", Product: "product", TransportID: "transport_id"}, nil},
		{"serial device device:device transport_id:transport_id", &Device{Serial: "serial", Device: "device", TransportID: "transport_id"}, nil},
		{"serial device random_prop:prop", &Device{Serial: "serial"}, nil},
	} {
		device, err := parseDevice(tc.line)
		if diff := cmp.Diff(device, tc.wantedDevice); diff != "" {
			t.Errorf("failed to parse device from %q correctly (-got +want):\n%s", tc.line, diff)
		}
		if !errors.Is(err, tc.wantedErr) {
			t.Errorf("failed to parse error from %q correctly (got: %q, want: %q)", tc.line, err, tc.wantedErr)
		}
	}
}
