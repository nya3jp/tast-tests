// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"

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
	var pciDevs []pci.Device
	var usbDevs []usb.Device
	for _, d := range res.Devices {
		if d.BusInfo.PciBusInfo != nil {
			pd := d.BusInfo.PciBusInfo
			pciDevs = append(pciDevs, pci.Device{
				VendorID: fmt.Sprintf("%04x", pd.VendorID),
				DeviceID: fmt.Sprintf("%04x", pd.DeviceID),
				Vendor:   d.VendorName,
				Device:   d.ProductName,
				Class:    fmt.Sprintf("%02x%02x", pd.ClassID, pd.SubClassID),
				ProgIf:   fmt.Sprintf("%02x", pd.ProgIfID),
				Driver:   pd.Driver,
			})
		} else if d.BusInfo.UsbBusInfo != nil {
			udi := d.BusInfo.UsbBusInfo
			udo := usb.Device{
				VendorID:   fmt.Sprintf("%04x", udi.VendorID),
				ProdID:     fmt.Sprintf("%04x", udi.ProductID),
				DeviceName: d.VendorName + " " + d.ProductName,
				Cls:        fmt.Sprintf("%02x", udi.ClassID),
				Sub:        fmt.Sprintf("%02x", udi.SubClassID),
				Prot:       fmt.Sprintf("%02x", udi.ProtocolID),
			}
			for _, ifc := range udi.Interfaces {
				udo.Interfaces = append(udo.Interfaces, usb.Interface{
					InterfaceNumber: fmt.Sprintf("%x", ifc.InterfaceNumber),
					Cls:             fmt.Sprintf("%02x", ifc.ClassID),
					Sub:             fmt.Sprintf("%02x", ifc.SubClassID),
					Prot:            fmt.Sprintf("%02x", ifc.ProtocolID),
					Driver:          ifc.Driver,
				})
			}
			usbDevs = append(usbDevs, udo)
		} else {
			s.Fatal("Unknown types of bus devices")
		}
	}

	ePci, err := pci.ExpectedDevices(ctx)
	if err != nil {
		s.Fatal("Failed to get expected pci devices: ", err)
	}
	pci.Sorted(pciDevs)
	if d := cmp.Diff(ePci, pciDevs); d != "" {
		s.Fatal("Pci devices validation failed (-expected + got): ", d)
	}

	eUsb, err := usb.ExpectedDevices(ctx)
	if err != nil {
		s.Fatal("Failed to get expected usb devices: ", err)
	}
	usb.Sorted(usbDevs)
	if d := cmp.Diff(eUsb, usbDevs); d != "" {
		s.Fatal("Usb devices validation failed (-expected + got): ", d)
	}
}

type pciBusInfo struct {
	ClassID    uint8   `json:"class_id"`
	SubClassID uint8   `json:"subclass_id"`
	ProgIfID   uint8   `json:"prog_if_id"`
	VendorID   uint16  `json:"vendor_id"`
	DeviceID   uint16  `json:"device_id"`
	Driver     *string `json:"driver"`
}

type usbInterfaceInfo struct {
	InterfaceNumber uint8   `json:"interface_number"`
	ClassID         uint8   `json:"class_id"`
	SubClassID      uint8   `json:"subclass_id"`
	ProtocolID      uint8   `json:"protocol_id"`
	Driver          *string `json:"driver"`
}

type usbBusInfo struct {
	ClassID    uint8              `json:"class_id"`
	SubClassID uint8              `json:"subclass_id"`
	ProtocolID uint8              `json:"protocol_id"`
	VendorID   uint16             `json:"vendor_id"`
	ProductID  uint16             `json:"product_id"`
	Interfaces []usbInterfaceInfo `json:"interfaces"`
}

type busInfo struct {
	PciBusInfo *pciBusInfo `json:"pci_bus_info"`
	UsbBusInfo *usbBusInfo `json:"usb_bus_info"`
}

type busDevice struct {
	VendorName  string  `json:"vendor_name"`
	ProductName string  `json:"product_name"`
	DeviceClass string  `json:"device_class"`
	BusInfo     busInfo `json:"bus_info"`
}

type busResult struct {
	Devices []busDevice `json:"devices"`
}
