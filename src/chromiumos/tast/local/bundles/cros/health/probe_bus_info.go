// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/local/jsontypes"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type interfaces struct {
	ClassID        jsontypes.Int32  `json:"class_id"`
	Driver         jsontypes.String `json:"driver"`
	ProtocolID     jsontypes.Int32  `json:"protocol_id"`
	SubClassID     jsontypes.Int32  `json:"subclass_id"`
	IntefaceNumber jsontypes.Int32  `json:"interface_number"`
}
type usbBusInfo struct {
	ClassID    jsontypes.Int32 `json:"class_id"`
	Interface  []interfaces    `json:"interfaces"`
	ProductID  jsontypes.Int32 `json:"product_id"`
	ProtocolID jsontypes.Int32 `json:"protocol_id"`
	SubClassID jsontypes.Int32 `json:"subclass_id"`
	VendorID   jsontypes.Int32 `json:"vendor_id"`
}
type pciBusInfo struct {
	ClassID    jsontypes.Int32  `json:"class_id"`
	DeviceID   jsontypes.Int32  `json:"device_id"`
	Driver     jsontypes.String `json:"driver"`
	ProofID    jsontypes.Int32  `json:"prog_if_id"`
	SubClassID jsontypes.Int32  `json:"subclass_id"`
	VendorID   jsontypes.Int32  `json:"vendor_id"`
}
type thunderboltInterfaces struct {
	Authorized      jsontypes.Bool   `json:"authorized"`
	DeviceFwVersion jsontypes.String `json:"device_fw_version"`
	DeviceName      jsontypes.String `json:"device_name"`
	DeviceType      jsontypes.String `json:"device_type"`
	DeviceUUID      jsontypes.String `json:"device_uuid"`
	RxSpeedGbs      jsontypes.String `json:"rx_speed_gbs"`
	TxSpeedGbs      jsontypes.String `json:"tx_speed_gbs"`
	VendorName      jsontypes.String `json:"vendor_name"`
}
type thunderboltBusInfo struct {
	SecurityLevel         jsontypes.String        `json:"security_level"`
	ThunderboltInterfaces []thunderboltInterfaces `json:"thunderbolt_interfaces"`
}
type busInfo struct {
	PciBusInfo         pciBusInfo         `json:"pci_bus_info"`
	UsbBusInfo         usbBusInfo         `json:"usb_bus_info"`
	ThunderboltBusInfo thunderboltBusInfo `json:"thunderbolt_bus_info"`
}

type devices struct {
	BusInfo     busInfo           `json:"bus_info"`
	DeviceClass *jsontypes.String `json:"device_class"`
	ProductName *jsontypes.String `json:"product_name"`
	VendorName  *jsontypes.String `json:"vendor_name"`
}
type deviceInfo struct {
	Devices []devices `json:"devices"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbeBUSInfo,
		Desc: "Check that we can probe cros_healthd for bus info and TBT data",
		Contacts: []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com",
			"pmoy@google.com",
			"cros-tdm@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		HardwareDeps: hwdep.D(hwdep.Model("brya")),
		Fixture:      "crosHealthdRunning",
	})
}

func validateTBTInterfacesData(tbtinterface thunderboltBusInfo) error {
	for _, interfaces := range tbtinterface.ThunderboltInterfaces {
		if !interfaces.Authorized {
			return errors.New("failed to authorize the TBT device")
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

func validateBUSData(bus deviceInfo) error {
	checkTBT := false
	if len(bus.Devices) < 1 {
		return errors.Errorf("invalid Devices, got %d; want 1+", len(bus.Devices))
	}
	for _, devices := range bus.Devices {

		if devices.BusInfo.ThunderboltBusInfo.SecurityLevel != "" {

			if err := validateTBTInterfacesData(devices.BusInfo.ThunderboltBusInfo); err != nil {
				return errors.Wrap(err, "failed to verify TBT interfaces")
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
			checkTBT = true
		}
	}
	if !checkTBT {
		return errors.New(" TBT devices not connected ")
	}
	return nil
}

func ProbeBUSInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryBus}
	var device deviceInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &device); err != nil {
		s.Fatal("Failed to get bus telemetry info: ", err)
	}
	if err := validateBUSData(device); err != nil {
		s.Fatal("Failed to validate Bus data: ", err)
	}
}
