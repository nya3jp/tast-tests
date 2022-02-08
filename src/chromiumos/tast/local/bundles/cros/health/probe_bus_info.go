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
	"chromiumos/tast/local/bundles/cros/typec/typecutils"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProbeBusInfo,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Check that we can probe cros_healthd for bus info",
		Contacts: []string{
			"cros-tdm-tpe-eng@google.com",
			"pathan.jilani@intel.com",
			"intel-chrome-system-automation-team@intel.com",
		},
		Attr: []string{"group:mainline"},
		// TODO(b/200837194): Remove this after the volteer2 issue fix.
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("volteer2")),
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
		Params: []testing.Param{{
			Val: false,
		}, {
			Name:      "thunderbolt",
			ExtraAttr: []string{"informational"},
			Val:       true,
			// TODO(b/207569436): Define hardware dependency and get rid of hard-coding the models.
			ExtraHardwareDeps: hwdep.D(hwdep.Model("brya", "redrix", "kano", "anahera", "primus", "crota")),
		}, {
			Name:              "volteer2",
			ExtraAttr:         []string{"informational"},
			Val:               false,
			ExtraHardwareDeps: hwdep.D(hwdep.Model("volteer2")),
		}},
	})
}

func ProbeBusInfo(ctx context.Context, s *testing.State) {
	isDeviceConnected := false
	isvProSupports := s.Param().(bool)
	if isvProSupports {
		// Checking whether the Thunderbolt device is connected or not.
		port, _ := typecutils.CheckPortsForTBTPartner(ctx)
		if port != -1 {
			//TODO(b/209385206): For accesing the Thunderbolt device we have to disable the data protection access from UI.
			isDeviceConnected = true
		}
	}

	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryBus}
	var res busResult
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &res); err != nil {
		s.Fatal("Failed to get bus telemetry info: ", err)
	}

	var pciDevs []busDevice
	var usbDevs []busDevice
	var tbtDevs []busDevice
	for _, d := range res.Devices {
		if d.BusInfo.PCIBusInfo != nil {
			pciDevs = append(pciDevs, d)
		} else if d.BusInfo.USBBusInfo != nil {
			usbDevs = append(usbDevs, d)
		} else if d.BusInfo.ThunderboltBusInfo != nil {
			tbtDevs = append(tbtDevs, d)
		} else {
			s.Fatal("Unknown types of bus devices: ", d)
		}
	}

	if isvProSupports {
		if err := validateThundeboltDevices(tbtDevs, isDeviceConnected); err != nil {
			s.Fatal("Failed to validate Thunderbolt devices: ", err)
		}
		return
	}

	if err := validatePCIDevices(ctx, pciDevs); err != nil {
		s.Fatal("PCI validation failed: ", err)
	}
	if err := validateUSBDevices(ctx, usbDevs); err != nil {
		s.Fatal("USB validation failed: ", err)
	}

}

// validatePCIDevices validates the PCI devices with the expected PCI
// devices extracted by the "lspci" command.
func validatePCIDevices(ctx context.Context, devs []busDevice) error {
	var got []pci.Device
	for _, d := range devs {
		pd := d.BusInfo.PCIBusInfo
		// TODO:(b/199683963): Validation of busDevice.DeviceClass is skipped.
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
		return errors.Errorf("unexpected PCI device data, (-expected + got): %s", d)
	}
	return nil
}

// validateUSBDevices validates the USB devices with the expected USB
// devices extracted by the "usb-devices" and the "lsusb" commands.
func validateUSBDevices(ctx context.Context, devs []busDevice) error {
	var got []usb.Device
	for _, d := range devs {
		udIn := d.BusInfo.USBBusInfo
		// TODO:(b/199683963): Validation of busDevice.DeviceClass is skipped.
		udOut := usb.Device{
			VendorID:    fmt.Sprintf("%04x", udIn.VendorID),
			ProdID:      fmt.Sprintf("%04x", udIn.ProductID),
			VendorName:  d.VendorName,
			ProductName: d.ProductName,
			Class:       fmt.Sprintf("%02x", udIn.ClassID),
			SubClass:    fmt.Sprintf("%02x", udIn.SubClassID),
			Protocol:    fmt.Sprintf("%02x", udIn.ProtocolID),
		}
		for _, ifc := range udIn.Interfaces {
			udOut.Interfaces = append(udOut.Interfaces, usb.Interface{
				InterfaceNumber: ifc.InterfaceNumber,
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
		return errors.Errorf("unexpected USB device data, (-expected + got): %s", d)
	}
	return nil
}

func validateThundeboltDevices(devs []busDevice, isDeviceConnected bool) error {
	for _, devices := range devs {
		if (devices.BusInfo.ThunderboltBusInfo.SecurityLevel) == "" {
			return errors.New("failed to enable SecurityLevel")
		}
		checkInterfacesDetcted := false
		if isDeviceConnected {
			for _, interfaces := range devices.BusInfo.ThunderboltBusInfo.ThunderboltInterfaces {
				checkInterfacesDetcted = true
				if !interfaces.Authorized {
					return errors.New("failed to authorize the Thunderbolt device")
				}
				if interfaces.DeviceFwVersion == "" {
					return errors.New("failed to get DeviceFwVersion")
				}
				if interfaces.DeviceName == "" {
					return errors.New("failed to get DeviceName")
				}
				if interfaces.DeviceType == "" {
					return errors.New("failed to get DeviceType")
				}
				if interfaces.DeviceUUID == "" {
					return errors.New("failed to get DeviceUUID")
				}
				if interfaces.RxSpeedGbs == "" {
					return errors.New("failed to get RxSpeedGbs")
				}
				if interfaces.TxSpeedGbs == "" {
					return errors.New("failed to get TxSpeedGbs")
				}
				if interfaces.VendorName == "" {
					return errors.New("failed to get VendorName")
				}
			}
		}

		if isDeviceConnected && !checkInterfacesDetcted {
			return errors.New("failed to get Thunderbolt device data when the device is connected")

		}

		if (devices.DeviceClass) == "" {
			return errors.New("failed to get Thunderbolt DeviceClass")
		}
		if (devices.ProductName) == "" {
			return errors.New("failed to get Thunderbolt ProductName")
		}
		if (devices.VendorName) == "" {
			return errors.New("failed to get Thunderbolt VendorName")
		}
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
	PCIBusInfo         *pciBusInfo         `json:"pci_bus_info"`
	USBBusInfo         *usbBusInfo         `json:"usb_bus_info"`
	ThunderboltBusInfo *thunderboltBusInfo `json:"thunderbolt_bus_info"`
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

// thunderboltInterfaceInfo represents the ThunderboltInterfaces in cros-healthd mojo
// interface.
type thunderboltInterfaceInfo struct {
	Authorized      bool   `json:"authorized"`
	DeviceFwVersion string `json:"device_fw_version"`
	DeviceName      string `json:"device_name"`
	DeviceType      string `json:"device_type"`
	DeviceUUID      string `json:"device_uuid"`
	RxSpeedGbs      string `json:"rx_speed_gbs"`
	TxSpeedGbs      string `json:"tx_speed_gbs"`
	VendorName      string `json:"vendor_name"`
}

// thunderboltBusInfo represents the ThunderboltBusInfo in cros-healthd mojo interface.
type thunderboltBusInfo struct {
	SecurityLevel         string                     `json:"security_level"`
	ThunderboltInterfaces []thunderboltInterfaceInfo `json:"thunderbolt_interfaces"`
}
