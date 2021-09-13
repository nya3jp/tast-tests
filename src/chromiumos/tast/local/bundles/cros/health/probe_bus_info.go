// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"

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
			"cros-tdm-tpe-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		HardwareDeps: hwdep.D(hwdep.Model("brya")),
		Fixture:      "crosHealthdRunning",
	})
}

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
	}
	return nil
}

func ProbeBusInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryBus}
	var device deviceInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &device); err != nil {
		s.Fatal("Failed to get bus telemetry info: ", err)
	}
	if err := validateBusData(device); err != nil {
		s.Fatal("Failed to validate Bus data: ", err)
	}
}
