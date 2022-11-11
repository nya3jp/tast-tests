// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/health/pci"
	"chromiumos/tast/local/bundles/cros/health/types"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/local/typecutils"
	"chromiumos/tast/local/usbutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type busInfoTestParams struct {
	// Whether to check thunderbolt devices.
	checkThunderbolt bool
	// Workaround for b/200837194 to skip checking PCI ProgIf field.
	checkProgIf bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProbeBusInfo,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check that we can probe cros_healthd for bus info",
		Contacts: []string{
			"cros-tdm-tpe-eng@google.com",
			"pathan.jilani@intel.com",
			"intel-chrome-system-automation-team@intel.com",
		},
		BugComponent: "b:982097",
		Attr:         []string{"group:mainline"},
		Vars:         []string{"ui.signinProfileTestExtensionManifestKey"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
		Params: []testing.Param{{
			Val: busInfoTestParams{
				checkThunderbolt: false,
				checkProgIf:      false,
			},
		}, {
			Name: "thunderbolt",
			Val: busInfoTestParams{
				checkThunderbolt: true,
				checkProgIf:      false,
			},
			ExtraData:         []string{"testcert.p12"},
			ExtraHardwareDeps: hwdep.D(hwdep.ChromeEC()),
		}, {
			// TODO(b/200837194): Remove this after the volteer2 issue fix.
			Name:      "progif",
			ExtraAttr: []string{"informational"},
			Val: busInfoTestParams{
				checkThunderbolt: false,
				checkProgIf:      true,
			},
		}},
	})
}

func ProbeBusInfo(ctx context.Context, s *testing.State) {
	isDeviceConnected := false
	isThunderboltSupport := false
	testParam := s.Param().(busInfoTestParams)

	if testParam.checkThunderbolt {
		// Checking whether device supports thunderbolt or not.
		outFiles, err := testexec.CommandContext(ctx, "ls", "/sys/bus/thunderbolt/devices").Output()
		if err != nil {
			s.Log("Failed to execute ls /sys/bus/thunderbolt/devices/ command: ", err)
		}
		requiredFiles := []string{"0-0", "1-0", "domain0", "domain1"}
		for _, file := range requiredFiles {
			if strings.Contains(string(outFiles), file) {
				isThunderboltSupport = true
			}
		}
		// Checking whether the Thunderbolt device is connected or not.
		port, _ := typecutils.CheckPortsForTBTPartner(ctx)
		if port != -1 {
			// For accesing the Thunderbolt device we have to disable the data protection access from UI.
			portStr := strconv.Itoa(port)
			if err := testexec.CommandContext(ctx, "ectool", "pdcontrol", "suspend", portStr).Run(); err != nil {
				s.Fatal("Failed to simulate unplug: ", err)
			}
			defer func(ctx context.Context) {
				if err := testexec.CommandContext(ctx, "ectool", "pdcontrol", "resume", portStr).Run(); err != nil {
					s.Error("Failed to perform replug: ", err)
				}
			}(ctx)
			// Get to the Chrome login screen.
			cr, err := chrome.New(ctx,
				chrome.DeferLogin(),
				chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))
			if err != nil {
				s.Fatal("Failed to start Chrome at login screen: ", err)
			}
			defer cr.Close(ctx)

			if err := typecutils.EnablePeripheralDataAccess(ctx, s.DataPath("testcert.p12")); err != nil {
				s.Fatal("Failed to enable peripheral data access setting: ", err)
			}

			if err := cr.ContinueLogin(ctx); err != nil {
				s.Fatal("Failed to login: ", err)
			}

			if err := testexec.CommandContext(ctx, "ectool", "pdcontrol", "resume", portStr).Run(); err != nil {
				s.Fatal("Failed to simulate replug: ", err)
			}

			err = testing.Poll(ctx, func(ctx context.Context) error {
				return typecutils.CheckTBTDevice(true)
			}, &testing.PollOptions{Timeout: 20 * time.Second})
			if err != nil {
				s.Fatal("Failed to verify Thunderbolt device connected: ", err)
			}

			isDeviceConnected = true
		}
	}

	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryBus}
	var res busResult
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &res); err != nil {
		s.Fatal("Failed to get bus telemetry info: ", err)
	}

	var pciDevs []types.BusDevice
	var usbDevs []types.BusDevice
	var tbtDevs []types.BusDevice
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

	if testParam.checkThunderbolt {
		if isThunderboltSupport {
			if err := validateThundeboltDevices(ctx, tbtDevs, isDeviceConnected); err != nil {
				s.Fatal("Failed to validate Thunderbolt devices: ", err)
			}
		} else {
			if len(tbtDevs) > 0 {
				s.Fatal("Failed to validate empty Thunderbolt data")
			}
		}
		return
	}

	if err := validatePCIDevices(ctx, pciDevs, testParam.checkProgIf); err != nil {
		s.Fatal("PCI validation failed: ", err)
	}
	if err := validateUSBDevices(ctx, usbDevs); err != nil {
		s.Fatal("USB validation failed: ", err)
	}

}

// validatePCIDevices validates the PCI devices with the expected PCI
// devices extracted by the "lspci" command.
func validatePCIDevices(ctx context.Context, devs []types.BusDevice, checkProgIf bool) error {
	var got []pci.Device
	for _, d := range devs {
		pciBusInfo := d.BusInfo.PCIBusInfo
		// TODO:(b/199683963): Validation of types.BusDevice.DeviceClass is skipped.
		pd := pci.Device{
			VendorID: fmt.Sprintf("%04x", pciBusInfo.VendorID),
			DeviceID: fmt.Sprintf("%04x", pciBusInfo.DeviceID),
			Vendor:   d.VendorName,
			Device:   d.ProductName,
			Class:    fmt.Sprintf("%02x%02x", pciBusInfo.ClassID, pciBusInfo.SubClassID),
			ProgIf:   fmt.Sprintf("%02x", pciBusInfo.ProgIfID),
			Driver:   pciBusInfo.Driver,
		}
		if !checkProgIf {
			pd.ProgIf = "(skip)"
		}
		got = append(got, pd)
	}
	pci.Sort(got)
	exp, err := pci.ExpectedDevices(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get expected devices")
	}
	if !checkProgIf {
		for i := range exp {
			exp[i].ProgIf = "(skip)"
		}
	}
	if d := cmp.Diff(exp, got); d != "" {
		return errors.Errorf("unexpected PCI device data, (-expected + got): %s", d)
	}
	return nil
}

// validateUSBDevices validates the USB devices with the expected USB
// devices extracted by the "usb-devices" and the "lsusb" commands.
func validateUSBDevices(ctx context.Context, devs []types.BusDevice) error {
	var got []usbutil.Device
	for _, d := range devs {
		udIn := d.BusInfo.USBBusInfo
		// TODO:(b/199683963): Validation of types.BusDevice.DeviceClass is skipped.
		udOut := usbutil.Device{
			VendorID:    fmt.Sprintf("%04x", udIn.VendorID),
			ProdID:      fmt.Sprintf("%04x", udIn.ProductID),
			VendorName:  d.VendorName,
			ProductName: d.ProductName,
			Class:       fmt.Sprintf("%02x", udIn.ClassID),
			SubClass:    fmt.Sprintf("%02x", udIn.SubClassID),
			Protocol:    fmt.Sprintf("%02x", udIn.ProtocolID),
		}
		for _, ifc := range udIn.Interfaces {
			udOut.Interfaces = append(udOut.Interfaces, usbutil.Interface{
				InterfaceNumber: ifc.InterfaceNumber,
				Class:           fmt.Sprintf("%02x", ifc.ClassID),
				SubClass:        fmt.Sprintf("%02x", ifc.SubClassID),
				Protocol:        fmt.Sprintf("%02x", ifc.ProtocolID),
				Driver:          ifc.Driver,
			})
		}
		if udIn.FwupdFirmwareVersionInfo != nil {
			udOut.FwupdFirmwareVersionInfo = &usbutil.FwupdFirmwareVersionInfo{
				Version:       udIn.FwupdFirmwareVersionInfo.Version,
				VersionFormat: udIn.FwupdFirmwareVersionInfo.VersionFormat,
			}
		}
		got = append(got, udOut)
	}
	usbutil.Sort(got)
	exp, err := usbutil.AttachedDevices(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get expected devices")
	}
	if d := cmp.Diff(exp, got); d != "" {
		return errors.Errorf("unexpected USB device data, (-expected + got): %s", d)
	}
	return nil
}

func validateThundeboltDevices(ctx context.Context, devs []types.BusDevice, isDeviceConnected bool) error {
	checkInterfacesDetected := false
	productName, err := ioutil.ReadFile("/sys/bus/thunderbolt/devices/0-0/device_name")
	if err != nil {
		testing.ContextLog(ctx, "Failed to read thunderbolt device name")
	}
	vendorName, err := ioutil.ReadFile("/sys/bus/thunderbolt/devices/0-0/vendor_name")
	if err != nil {
		testing.ContextLog(ctx, "Failed to read thunderbolt vendor name")
	}
	for _, devices := range devs {
		if (devices.BusInfo.ThunderboltBusInfo.SecurityLevel) == "" {
			return errors.New("failed to enable SecurityLevel")
		}
		if isDeviceConnected {
			for _, interfaces := range devices.BusInfo.ThunderboltBusInfo.ThunderboltInterfaces {
				checkInterfacesDetected = true
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
			}
		}

		if (devices.DeviceClass) == "" {
			return errors.New("failed to get Thunderbolt DeviceClass")
		}

		productName := strings.TrimSpace(string(productName))
		if devices.ProductName != productName {
			return errors.Errorf("failed to get correct Thunderbolt ProductName: got %q; want %q", productName, devices.ProductName)
		}

		vendorName := strings.TrimSpace(string(vendorName))
		if devices.VendorName != vendorName {
			return errors.Errorf("failed to get correct Thunderbolt VendorName: got %q; want %q", vendorName, devices.VendorName)
		}

	}

	if isDeviceConnected && !checkInterfacesDetected {
		return errors.New("failed to get Thunderbolt device data when the device is connected")
	}

	return nil
}

// busResult represents the BusResult in cros-healthd mojo interface.
type busResult struct {
	Devices []types.BusDevice `json:"devices"`
}
