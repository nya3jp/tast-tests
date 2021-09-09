// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/health/pci"
	"chromiumos/tast/local/bundles/cros/health/usb"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbeBusInfo,
		Desc: "Check that we can probe cros_healthd for bus info",
		Contacts: []string{
			"cros-tdm-tpe-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func ProbeBusInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryBus}
	var res busResult
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &res); err != nil {
		s.Fatal("Failed to get bus telemetry info: ", err)
	}
	var pciDevs []busDevice
	var usbDevs []busDevice
	for _, d := range res.Devices {
		if d.BusInfo.PCIBusInfo != nil {
			pciDevs = append(pciDevs, d)
		} else if d.BusInfo.USBBusInfo != nil {
			usbDevs = append(usbDevs, d)
		} else {
			s.Fatal("Unknown types of bus devices: ", d)
		}
	}

	if err := validatePciDevices(ctx, pciDevs); err != nil {
		s.Fatal("Pci validation failed: ", err)
	}
	if err := validateUsbDevices(ctx, usbDevs); err != nil {
		s.Fatal("Usb validation failed: ", err)
	}
}

func validatePciDevices(ctx context.Context, devs []busDevice) error {
	var got []pci.Device
	for _, d := range devs {
		pd := d.BusInfo.PCIBusInfo
		got = append(got, pci.Device{
			VendorID: fmt.Sprintf("%04x", pd.VendorID),
			DeviceID: fmt.Sprintf("%04x", pd.DeviceID),
			Vendor:   d.VendorName,
			Device:   d.ProductName,
			Class:    fmt.Sprintf("%02x%02x", pd.ClassID, pd.SubClassID),
			ProgIf:   fmt.Sprintf("%02x", pd.ProgIfID),
			Driver:   pd.Driver,
		})
	}
	pci.Sort(got)
	exp, err := pci.ExpectedDevices(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get expected devices")
	}
	if d := cmp.Diff(exp, got); d != "" {
		return errors.Errorf("(-expected + got): %s", d)
	}
	return nil
}

func validateUsbDevices(ctx context.Context, devs []busDevice) error {
	var got []usb.Device
	for _, d := range devs {
		udIn := d.BusInfo.USBBusInfo
		udOut := usb.Device{
			VendorID:   fmt.Sprintf("%04x", udIn.VendorID),
			ProdID:     fmt.Sprintf("%04x", udIn.ProductID),
			DeviceName: d.VendorName + " " + d.ProductName,
			Class:      fmt.Sprintf("%02x", udIn.ClassID),
			SubClass:   fmt.Sprintf("%02x", udIn.SubClassID),
			Protocol:   fmt.Sprintf("%02x", udIn.ProtocolID),
		}
		for _, ifc := range udIn.Interfaces {
			udOut.Interfaces = append(udOut.Interfaces, usb.Interface{
				InterfaceNumber: fmt.Sprintf("%x", ifc.InterfaceNumber),
				Class:           fmt.Sprintf("%02x", ifc.ClassID),
				SubClass:        fmt.Sprintf("%02x", ifc.SubClassID),
				Protocol:        fmt.Sprintf("%02x", ifc.ProtocolID),
				Driver:          ifc.Driver,
			})
		}
		got = append(got, udOut)
	}
	usb.Sort(got)
	exp, err := usb.ExpectedDevices(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get expected devices")
	}
	if d := cmp.Diff(exp, got); d != "" {
		return errors.Errorf("(-expected + got): %s", d)
	}
	return nil
}

// busResult represents the BusResult in cros-healthd mojo interface.
type busResult struct {
	Devices []busDevice `json:"devices"`
}

// busDevice represents the BusDevice in cros-healthd mojo interface.
type busDevice struct {
	VendorName  string  `json:"vendor_name"`
	ProductName string  `json:"product_name"`
	DeviceClass string  `json:"device_class"`
	BusInfo     busInfo `json:"bus_info"`
}

// busInfo represents the BusInfo in cros-healthd mojo interface.
type busInfo struct {
	PCIBusInfo *pciBusInfo `json:"pci_bus_info"`
	USBBusInfo *usbBusInfo `json:"usb_bus_info"`
}

// pciBusInfo represents the PciBusInfo in cros-healthd mojo interface.
type pciBusInfo struct {
	ClassID    uint8   `json:"class_id"`
	SubClassID uint8   `json:"subclass_id"`
	ProgIfID   uint8   `json:"prog_if_id"`
	VendorID   uint16  `json:"vendor_id"`
	DeviceID   uint16  `json:"device_id"`
	Driver     *string `json:"driver"`
}

// usbBusInfo represents the UsbBusInfo in cros-healthd mojo interface.
type usbBusInfo struct {
	ClassID    uint8              `json:"class_id"`
	SubClassID uint8              `json:"subclass_id"`
	ProtocolID uint8              `json:"protocol_id"`
	VendorID   uint16             `json:"vendor_id"`
	ProductID  uint16             `json:"product_id"`
	Interfaces []usbInterfaceInfo `json:"interfaces"`
}

// usbInterfaceInfo represents the UsbInterfaceInfo in cros-healthd mojo
// interface.
type usbInterfaceInfo struct {
	InterfaceNumber uint8   `json:"interface_number"`
	ClassID         uint8   `json:"class_id"`
	SubClassID      uint8   `json:"subclass_id"`
	ProtocolID      uint8   `json:"protocol_id"`
	Driver          *string `json:"driver"`
}
