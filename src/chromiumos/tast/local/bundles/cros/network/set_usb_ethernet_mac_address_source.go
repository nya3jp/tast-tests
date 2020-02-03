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
		Attr:         []string{"informational"},
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
//   3) DUT must have both `ethernet_mac0` and `dock_mac` read only VPD fields
//      with MAC addresses.
// TODO(crbug/1012367): Add these hardware dependencies when Tast will support them.
func SetUSBEthernetMACAddressSource(ctx context.Context, s *testing.State) {
	readMACFromVPD := func(vpd string) string {
		bytes, err := ioutil.ReadFile(filepath.Join("/sys/firmware/vpd/ro", vpd))
		if err != nil {
			s.Fatalf("Failed to read VPD field %s file: %v", vpd, err)
		}
		return strings.ToLower(string(bytes))
	}
	ethMAC := readMACFromVPD("ethernet_mac0")
	dockMAC := readMACFromVPD("dock_mac")

	eth, name := func() (*shill.Device, string) {
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
			if !deviceProps.Has(shill.DevicePropertyEthernetBusType) {
				continue
			}
			busType, err := deviceProps.GetString(shill.DevicePropertyEthernetBusType)
			if err != nil {
				s.Fatal("Failed to get bus type: ", err)
			}

			iface, err := deviceProps.GetString(shill.DevicePropertyInterface)
			if err != nil {
				s.Fatal("Failed to get interface name: ", err)
			}

			if busType == "usb" {
				return device, iface
			}
		}
		s.Fatal("DUT does not have USB Ethernet adapter")
		return nil, ""
	}()

	s.Log("DUT has USB Ethernet adapter: ", eth)

	// We lose connectivity along the way here, and if that races with the
	// recover_duts network-recovery hooks, it may interrupt us.
	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		s.Fatal("Failed to lock the check network hook: ", err)
	}
	defer unlock()

	setMACSource := func(source string) {
		s.Log("Setting USB Ethernet MAC address source to ", source)
		if err := eth.SetUsbEthernetMacAddressSource(ctx, source); err != nil {
			s.Fatal("Can not set USB Ethernet MAC address source: ", err)
		} else {
			s.Log("Successfully set USB Ethernet MAC address source: ", source)
		}
	}

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

	verifyProperty := func(prop string, actualValue interface{}, expectedValue string) {
		if str, ok := actualValue.(string); !ok || str != expectedValue {
			s.Fatalf("Property %s changed unexpectedly: got %v, want %v", prop, actualValue, expectedValue)
		}
	}

	verify := func(source, expectedMAC string) {
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

		setMACSource(source)

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		s.Log("Start watching PropertyChanged signals")
		vals, err := signalWatcher.WaitAll(ctx, shill.DevicePropertyAddress, shill.DevicePropertyEthernetMACSource)
		if err != nil {
			s.Fatal("Failed to wait expected changes: ", err)
		}

		verifyProperty(shill.DevicePropertyAddress, vals[0], strings.Replace(expectedMAC, ":", "", -1))
		verifyProperty(shill.DevicePropertyEthernetMACSource, vals[1], source)

		if mac := getMAC(); mac != expectedMAC {
			s.Fatalf("Can not verify MAC address change via `net` library, current MAC is %s vs %s expected", mac, expectedMAC)
		}
	}

	defer setMACSource("usb_adapter_mac")

	usbMAC := getMAC()

	for _, tc := range []struct {
		source      string
		expectedMAC string
	}{
		{"designated_dock_mac", dockMAC},
		{"builtin_adapter_mac", ethMAC},
		{"usb_adapter_mac", usbMAC},
	} {
		verify(tc.source, tc.expectedMAC)
	}
}
