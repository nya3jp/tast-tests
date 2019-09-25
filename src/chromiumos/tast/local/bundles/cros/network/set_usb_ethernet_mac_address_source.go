// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"io/ioutil"
	"net"
	"strings"
	"time"

	"chromiumos/tast/errors"
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
		Attr: []string{"informational"},
	})
}

// SetUSBEthernetMACAddressSource call `SetUsbEthernetMacAddressSource` device DBus
// function with all possible valid values.
//
// Test physical precondistions:
//   1) DUT must have connected at least one USB Ethernet adapter that support MAC
//      address change (e.g. r8152 driver).
//   2) DUT must not have connected USB Ethernet adapters that don't support MAC
//      address change (e.g. asix driver).
func SetUSBEthernetMACAddressSource(ctx context.Context, s *testing.State) {
	readMACFromVPD := func(vpd string) string {
		bytes, err := ioutil.ReadFile("/sys/firmware/vpd/ro/" + vpd)
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

		devices, err := manager.GetDevices(ctx)
		if err != nil {
			s.Fatal("Failed getting devices: ", err)
		}
		for _, devicePath := range devices {
			device, err := shill.NewDevice(ctx, devicePath)
			if err != nil {
				s.Fatal("Failed to create device: ", err)
			}
			props, err := device.GetProps(ctx)
			if err != nil {
				s.Fatal("Failed to load device properties: ", err)
			}

			busType, ok := props[shill.DevicePropEthernetBusType].(string)
			if !ok {
				continue
			}
			name, ok := props[shill.DevicePropName].(string)
			if !ok {
				continue
			}

			if busType == "usb" {
				return device, name
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
		if err := eth.SetMACSource(ctx, source); err != nil {
			s.Fatal("Can not set USB Ethernet MAC address source: ", err)
		} else {
			s.Log("Successfully set USB Ethernet MAC address source: ", source)
		}

		props, err := eth.GetProps(ctx)
		if err != nil {
			s.Fatal("Failed to load device properties: ", err)
		}

		newSource, ok := props[shill.DevicePropEthernetMACSource].(string)
		if !ok {
			s.Fatal("Failed to get USB Ethernet MAC address source property")
		}

		if newSource != source {
			s.Fatalf("Unexpected USB Ethernet MAC address source property %s, expecting %s ",
				newSource, source)
		}
	}

	getMAC := func() string {
		ifi, err := net.InterfaceByName(name)
		if err != nil {
			s.Fatal("Can not get interface by name: ", err)
		}
		if ifi.HardwareAddr == nil {
			s.Fatal("Interface MAC address is nil")
		}
		return strings.ToLower(ifi.HardwareAddr.String())
	}

	waitMAC := func(expectedMAC string) {
		s.Log("Waiting for MAC address to be equal to ", expectedMAC)
		err := testing.Poll(ctx, func(ctx context.Context) error {
			mac := getMAC()
			if mac == expectedMAC {
				return nil
			}
			return errors.Errorf("actual MAC %s is not equal to the expected MAC %s", mac, expectedMAC)
		}, &testing.PollOptions{Timeout: 5 * time.Second})
		if err != nil {
			s.Fatal("Fimed out waiting for changing MAC address: ", err)
		}
	}

	defer setMACSource("usb_adapter_mac")

	usbMAC := getMAC()

	setMACSource("designated_dock_mac")
	waitMAC(dockMAC)

	setMACSource("builtin_adapter_mac")
	waitMAC(ethMAC)

	setMACSource("usb_adapter_mac")
	waitMAC(usbMAC)
}
