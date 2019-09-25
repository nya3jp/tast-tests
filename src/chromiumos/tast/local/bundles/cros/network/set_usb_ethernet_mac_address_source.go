// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"net"

	"chromiumos/tast/local/network"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SetUsbEthernetMacAddressSource,
		Desc: "Test SetUsbEthernetMacAddressSource D-Bus method",
		Contacts: []string{
			"lamzin@google.com",
			"cros-networking@google.com",
		},
		Attr: []string{"informational"},
	})
}

func SetUsbEthernetMacAddressSource(ctx context.Context, s *testing.State) {
	primaryUsbEthernet, interfaceName := func() (*shill.Device, string) {
		manager, err := shill.NewManager(ctx)
		if err != nil {
			s.Fatal("Failed creating shill manager proxy: ", err)
		}

		devices, err := manager.GetDevices(ctx)
		if err != nil {
			s.Fatal("Failed getting devices: ", err)
		}
		for _, devicePath := range devices {
			device, err := shill.NewDevice(ctx, devicePath)
			if err != nil {
				s.Fatal("Failed to create device: ", err)
			}
			properties, err := device.GetProperties(ctx)
			if err != nil {
				s.Fatal("Failed to load device properties: ", err)
			}

			if properties["Ethernet.DeviceBusType"] == "usb" && properties["Ethernet.LinkUp"].(bool) {
				return device, properties["Interface"].(string)
			}
		}
		return nil, ""
	}()
	if primaryUsbEthernet == nil {
		s.Fatal("DUT does not have connected to the network USB Ethernet adapter")
	}

	s.Log("DUT has connected to the network USB Ethernet adapter: ", primaryUsbEthernet)

	// We lose connectivity along the way here, and if that races with the
	// recover_duts network-recovery hooks, it may interrupt us.
	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		s.Fatal("Failed to lock the check network hook: ", err)
	}
	defer unlock()

	setMacAddressSource := func(source string) {
		if err := primaryUsbEthernet.SetUsbEthernetMacAddressSource(ctx, source); err != nil {
			s.Fatal("Can not set USB Ethernet MAC address source: ", err)
		} else {
			s.Log("Successfuly set USB Ethernet MAC address source: ", source)
		}

		properties, err := primaryUsbEthernet.GetProperties(ctx)
		if err != nil {
			s.Fatal("Failed to load device properties: ", err)
		}
		if properties["Ethernet.UsbEthernetMacAddressSource"] != source {
			s.Fatalf("Unexpected UsbEthernetMacAddressSource property %s, expecting %s ",
				properties["Ethernet.UsbEthernetMacAddressSource"], source)
		}
	}

	getMacAddress := func() string {
		ifi, err := net.InterfaceByName(interfaceName)
		if err != nil {
			s.Fatal("Can not get interface by name: ", err)
		}
		if ifi.HardwareAddr == nil {
			s.Fatal("Interface MAC address is nil")
		}
		return string(ifi.HardwareAddr)
	}

	expectDifferentMacAddresses := func(a string, b string) {
		if a == b {
			s.Fatal("MAC address have not changed after changing MAC address source: ", a)
		}
	}

	setMacAddressSource("usb_adapter_mac")
	usbAdapterMac := getMacAddress()

	setMacAddressSource("designated_dock_mac")
	designatedDockMac := getMacAddress()
	expectDifferentMacAddresses(usbAdapterMac, designatedDockMac)

	setMacAddressSource("builtin_adapter_mac")
	builtinAdapterMac := getMacAddress()
	expectDifferentMacAddresses(designatedDockMac, builtinAdapterMac)

	setMacAddressSource("usb_adapter_mac")
	usbAdapterMac2 := getMacAddress()
	expectDifferentMacAddresses(builtinAdapterMac, usbAdapterMac2)

	if usbAdapterMac != usbAdapterMac2 {
		s.Fatalf("MAC address is not the same as was at the beginning of the test: %s vs %s",
			usbAdapterMac, usbAdapterMac2)
	}

}
