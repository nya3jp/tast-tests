// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package types provides interface types shared by healthd tast files.
package types

// BusDevice represents the BusDevice in cros-healthd mojo interface.
type BusDevice struct {
	VendorName  string  `json:"vendor_name"`
	ProductName string  `json:"product_name"`
	DeviceClass string  `json:"device_class"`
	BusInfo     BusInfo `json:"bus_info"`
}

// BusInfo represents the BusInfo in cros-healthd mojo interface.
type BusInfo struct {
	PCIBusInfo         *PCIBusInfo         `json:"pci_bus_info"`
	USBBusInfo         *USBBusInfo         `json:"usb_bus_info"`
	ThunderboltBusInfo *ThunderboltBusInfo `json:"thunderbolt_bus_info"`
}

// PCIBusInfo represents the PciBusInfo in cros-healthd mojo interface.
type PCIBusInfo struct {
	ClassID    uint8   `json:"class_id"`
	SubClassID uint8   `json:"subclass_id"`
	ProgIfID   uint8   `json:"prog_if_id"`
	VendorID   uint16  `json:"vendor_id"`
	DeviceID   uint16  `json:"device_id"`
	Driver     *string `json:"driver"`
}

// USBBusInfo represents the UsbBusInfo in cros-healthd mojo interface.
type USBBusInfo struct {
	ClassID                  uint8                     `json:"class_id"`
	SubClassID               uint8                     `json:"subclass_id"`
	ProtocolID               uint8                     `json:"protocol_id"`
	VendorID                 uint16                    `json:"vendor_id"`
	ProductID                uint16                    `json:"product_id"`
	Interfaces               []USBInterfaceInfo        `json:"interfaces"`
	FwupdFirmwareVersionInfo *FwupdFirmwareVersionInfo `json:"fwupd_firmware_version_info"`
}

// USBInterfaceInfo represents the UsbInterfaceInfo in cros-healthd mojo
// interface.
type USBInterfaceInfo struct {
	InterfaceNumber uint8   `json:"interface_number"`
	ClassID         uint8   `json:"class_id"`
	SubClassID      uint8   `json:"subclass_id"`
	ProtocolID      uint8   `json:"protocol_id"`
	Driver          *string `json:"driver"`
}

// FwupdFirmwareVersionInfo represents the FwupdFirmwareVersionInfo in
// cros-healthd mojo interface.
type FwupdFirmwareVersionInfo struct {
	Version       string `json:"version"`
	VersionFormat string `json:"version_format"`
}

// ThunderboltInterfaceInfo represents the ThunderboltInterfaces in cros-healthd mojo
// interface.
type ThunderboltInterfaceInfo struct {
	Authorized      bool   `json:"authorized"`
	DeviceFwVersion string `json:"device_fw_version"`
	DeviceName      string `json:"device_name"`
	DeviceType      string `json:"device_type"`
	DeviceUUID      string `json:"device_uuid"`
	RxSpeedGbs      string `json:"rx_speed_gbs"`
	TxSpeedGbs      string `json:"tx_speed_gbs"`
	VendorName      string `json:"vendor_name"`
}

// ThunderboltBusInfo represents the ThunderboltBusInfo in cros-healthd mojo interface.
type ThunderboltBusInfo struct {
	SecurityLevel         string                     `json:"security_level"`
	ThunderboltInterfaces []ThunderboltInterfaceInfo `json:"thunderbolt_interfaces"`
}
