// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"io/ioutil"
	"net"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SetUSBEthernetMACAddressSource,
		Desc: "Test that USB Ethernet adapter changes its MAC address",
		Contacts: []string{
			"lamzin@google.com",
			"cros-networking@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"wilco"},
	})
}

// SetUSBEthernetMACAddressSource call `SetUsbEthernetMacAddressSource` device DBus
// function with all possible valid values.
//
// Test physical preconditions:
//   1) DUT must have connected at least one USB Ethernet adapter that support MAC
//      address change (e.g. r8152 driver).
//   2) DUT must not have connected USB Ethernet adapters that don't support MAC
//      address change (e.g. asix driver).
//   3) DUT must have the`dock_mac` read only VPD field containing a MAC address.
//      The `ethernet_mac0` read only VPD field containing a MAC address is optional.
// TODO(crbug/1012367): Add these hardware dependencies when Tast will support them.
func SetUSBEthernetMACAddressSource(ctx context.Context, s *testing.State) {
	readMACFromVPD := func(vpd string) string {
		bytes, err := ioutil.ReadFile(filepath.Join("/sys/firmware/vpd/ro", vpd))
		if err != nil {
			s.Logf("Failed to read VPD field %s file: %v", vpd, err)
			return ""
		}
		return strings.ToLower(string(bytes))
	}

	if readMACFromVPD("dock_mac") == "" {
		s.Fatal("dock_mac VPD field is empty")
	}

	eth, name := func(ctx context.Context) (*shill.Device, string) {
		manager, err := shill.NewManager(ctx)
		if err != nil {
			s.Fatal("Failed creating shill manager proxy: ", err)
		}

		devices, props, err := manager.DevicesByTechnology(ctx, shill.TechnologyEthernet)
		if err != nil {
			s.Fatal("Failed getting devices: ", err)
		}
		for i, device := range devices {
			deviceProps := props[i]
			if !deviceProps.Has(shillconst.DevicePropertyEthernetBusType) {
				continue
			}
			busType, err := deviceProps.GetString(shillconst.DevicePropertyEthernetBusType)
			if err != nil {
				s.Fatal("Failed to get bus type: ", err)
			}

			iface, err := deviceProps.GetString(shillconst.DevicePropertyInterface)
			if err != nil {
				s.Fatal("Failed to get interface name: ", err)
			}

			if busType == "usb" {
				return device, iface
			}
		}
		s.Fatal("DUT does not have USB Ethernet adapter")
		return nil, ""
	}(ctx)

	s.Log("DUT has USB Ethernet adapter: ", eth)

	// We lose connectivity along the way here, and if that races with the
	// recover_duts network-recovery hooks, it may interrupt us.
	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		s.Fatal("Failed to lock the check network hook: ", err)
	}
	defer unlock()

	getMAC := func() string {
		ifi, err := net.InterfaceByName(name)
		if err != nil {
			s.Fatal("Cannot get interface by name: ", err)
		}
		if ifi.HardwareAddr == nil {
			s.Fatal("Interface MAC address is nil")
		}
		return strings.ToLower(ifi.HardwareAddr.String())
	}

	setMACSource := func(ctx context.Context, source string) {
		s.Log("Setting USB Ethernet MAC address source to ", source)
		if err := eth.SetUsbEthernetMacAddressSource(ctx, source); err != nil {
			s.Fatal("Can not set USB Ethernet MAC address source: ", err)
		} else {
			s.Log("Successfully set USB Ethernet MAC address source: ", source)
		}
	}

	verify := func(ctx context.Context, source, expectedMAC string) {
		verifyProperty := func(prop string, actualValue interface{}, expectedValue string) {
			if str, ok := actualValue.(string); !ok || str != expectedValue {
				s.Fatalf("Property %s changed unexpectedly: got %v, want %v", prop, actualValue, expectedValue)
			}
		}

		signalWatcher, err := eth.CreateWatcher(ctx)
		if err != nil {
			s.Fatal("Failed to observe the property changed being dismissed: ", err)
		}
		defer func() {
			if err := signalWatcher.Close(ctx); err != nil {
				s.Log("Failed to close device PropertyChanged watcher: ", err)
			}
		}()

		s.Log("Start changing MAC address source")

		setMACSource(ctx, source)

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		s.Log("Start watching PropertyChanged signals")
		vals, err := signalWatcher.WaitAll(ctx, shillconst.DevicePropertyAddress, shillconst.DevicePropertyEthernetMACSource)
		if err != nil {
			s.Fatal("Failed to wait expected changes: ", err)
		}

		verifyProperty(shillconst.DevicePropertyAddress, vals[0], strings.Replace(expectedMAC, ":", "", -1))
		verifyProperty(shillconst.DevicePropertyEthernetMACSource, vals[1], source)

		if mac := getMAC(); mac != expectedMAC {
			s.Fatalf("Can not verify MAC address change via `net` library, current MAC is %s vs %s expected", mac, expectedMAC)
		}
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	defer setMACSource(cleanupCtx, "usb_adapter_mac")

	type testCase struct {
		source      string
		expectedMAC string
	}

	tcs := []testCase{
		{"designated_dock_mac", readMACFromVPD("dock_mac")},
		{"usb_adapter_mac", getMAC()},
	}

	if ethernetMAC := readMACFromVPD("ethernet_mac0"); ethernetMAC == "" {
		s.Log("ethernet_mac0 VPD field is empty. DUT may not support such MAC address source")
	} else {
		tcs = append(tcs, testCase{"builtin_adapter_mac", ethernetMAC})
	}

	for _, tc := range tcs {
		verify(ctx, tc.source, tc.expectedMAC)
	}
}
