// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
<<<<<<< HEAD
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
=======

	"chromiumos/tast/errors"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type usbBusInterface struct {
	ClassID        int32  `json:"class_id"`
	Driver         string `json:"driver"`
	ProtocolID     int32  `json:"protocol_id"`
	SubClassID     int32  `json:"subclass_id"`
	IntefaceNumber int32  `json:"interface_number"`
}
type usbBusInfo struct {
	ClassID          int32             `json:"class_id"`
	UsbBusInterfaces []usbBusInterface `json:"interfaces"`
	ProductID        int32             `json:"product_id"`
	ProtocolID       int32             `json:"protocol_id"`
	SubClassID       int32             `json:"subclass_id"`
	VendorID         int32             `json:"vendor_id"`
}
type pciBusInfo struct {
	ClassID    int32  `json:"class_id"`
	DeviceID   int32  `json:"device_id"`
	Driver     string `json:"driver"`
	ProofID    int32  `json:"prog_if_id"`
	SubClassID int32  `json:"subclass_id"`
	VendorID   int32  `json:"vendor_id"`
}
type thunderboltInterface struct {
	Authorized      bool   `json:"authorized"`
	DeviceFwVersion string `json:"device_fw_version"`
	DeviceName      string `json:"device_name"`
	DeviceType      string `json:"device_type"`
	DeviceUUID      string `json:"device_uuid"`
	RxSpeedGbs      string `json:"rx_speed_gbs"`
	TxSpeedGbs      string `json:"tx_speed_gbs"`
	VendorName      string `json:"vendor_name"`
}
type thunderboltBusInfo struct {
	SecurityLevel         string                 `json:"security_level"`
	ThunderboltInterfaces []thunderboltInterface `json:"thunderbolt_interfaces"`
}
type busInfo struct {
	PciBusInfo         pciBusInfo         `json:"pci_bus_info"`
	UsbBusInfo         usbBusInfo         `json:"usb_bus_info"`
	ThunderboltBusInfo thunderboltBusInfo `json:"thunderbolt_bus_info"`
}

type device struct {
	BusInfo     busInfo `json:"bus_info"`
	DeviceClass *string `json:"device_class"`
	ProductName *string `json:"product_name"`
	VendorName  *string `json:"vendor_name"`
}
type deviceInfo struct {
	Devices []device `json:"devices"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbeBusInfo,
		Desc: "Check that we can probe cros_healthd for bus info and Thunderbolt data",
		Contacts: []string{"pathan.jilani@intel.com",
			"intel-chrome-system-automation-team@intel.com",
>>>>>>> 9036206bd490... health: add ProbeBusInfo
			"cros-tdm-tpe-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
<<<<<<< HEAD
=======
		HardwareDeps: hwdep.D(hwdep.Model("brya")),
>>>>>>> 9036206bd490... health: add ProbeBusInfo
		Fixture:      "crosHealthdRunning",
	})
}

<<<<<<< HEAD
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
		return errors.Errorf("unexpected USB device data, (-expected + got): %s", d)
=======
func validateThunderboltInterfacesData(tbtinterface thunderboltBusInfo) error {
	for _, interfaces := range tbtinterface.ThunderboltInterfaces {
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
	return nil
}

func validateBusData(bus deviceInfo) error {
	isExist := false
	if len(bus.Devices) < 1 {
		return errors.New("expected at least one bus device")
	}

	for _, devices := range bus.Devices {
		if devices.BusInfo.ThunderboltBusInfo.SecurityLevel != "" {
			if err := validateThunderboltInterfacesData(devices.BusInfo.ThunderboltBusInfo); err != nil {
				return errors.Wrap(err, "failed to verify Thunderbolt interfaces")
			}
			if *(devices.DeviceClass) == "" {
				return errors.New("failed to get Thunderbolt DeviceClass")
			}
			if *(devices.ProductName) == "" {
				return errors.New("failed to get Thunderbolt ProductName")
			}
			if *(devices.VendorName) == "" {
				return errors.New("failed to get Thunderbolt VendorName")
			}
			isExist = true
		}
	}
	if !isExist {
		return errors.New("Thunderbolt devices not connected ")
>>>>>>> 9036206bd490... health: add ProbeBusInfo
	}
	return nil
}

<<<<<<< HEAD
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
=======
func ProbeBusInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryBus}
	var device deviceInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &device); err != nil {
		s.Fatal("Failed to get bus telemetry info: ", err)
	}
	if err := validateBusData(device); err != nil {
		s.Fatal("Failed to validate Bus data: ", err)
	}
>>>>>>> 9036206bd490... health: add ProbeBusInfo
}
