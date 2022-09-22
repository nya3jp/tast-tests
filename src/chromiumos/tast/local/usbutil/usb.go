// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package usbutil provides USB utility functions for checking device information.
package usbutil

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Device represents a USB device.
type Device struct {
	VendorID                 string
	ProdID                   string
	VendorName               string
	ProductName              string
	Class                    string
	SubClass                 string
	Protocol                 string
	Interfaces               []Interface
	FwupdFirmwareVersionInfo *FwupdFirmwareVersionInfo
}

// Interface represents a USB interface.
type Interface struct {
	InterfaceNumber uint8
	Class           string
	SubClass        string
	Protocol        string
	Driver          *string
}

// FwupdFirmwareVersionInfo represents a firmware version information obtained from fwupd.
type FwupdFirmwareVersionInfo struct {
	Version       string
	VersionFormat string
}

// fwupdGetDevicesResponse is used for parsing the JSON response of the `fwupdmgr get-devices` command.
type fwupdGetDevicesResponse struct {
	Devices []fwupdDevice `json:"Devices"`
}

// fwupdDevice represents a device in fwupdGetDevicesResponse.
type fwupdDevice struct {
	GUID          []string `json:"Guid"`
	Serial        *string  `json:"Serial"`
	VendorID      *string  `json:"VendorId"`
	Version       *string  `json:"Version"`
	VersionFormat *string  `json:"VersionFormat"`
}

// For mocking.
var runCommand = func(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	return testexec.CommandContext(ctx, cmd, args...).Output(testexec.DumpLogOnError)
}

// For mocking.
var readFile = ioutil.ReadFile

// usbDevices returns a list of USB devices. Each device is represented as a
// list of string. Each string contains some attributes related to the device.
func usbDevices(ctx context.Context) ([][]string, error) {
	const usbDevicesPath = "/sys/kernel/debug/usb/devices"
	b, err := readFile(usbDevicesPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read file: %v", usbDevicesPath)
	}
	// /sys/kernel/debug/usb/devices looks like:
	//   [An empty line]
	//   T: Bus=01 Lev=00 Prnt=00 Port=00 Cnt=00 Dev#=  1 Spd=480 MxCh=16
	//   D: Ver= 2.00 Cls=09(hub  ) Sub=00 Prot=01 MxPS=64 #Cfgs=  1
	//   ...
	//   [Another empty line]
	//   T: ...
	//   D: ...
	//   ...
	// where an empty line represents start of device.
	var res [][]string
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		if sc.Text() == "" {
			res = append(res, []string{})
		} else {
			i := len(res) - 1
			res[i] = append(res[i], sc.Text())
		}
	}
	return res, nil
}

// deviceNames returns the vendor name and the product name of device with
// vendorID:prodID and busNumber:devNumber. The names are extracted from lsusb.
func deviceNames(ctx context.Context, vendorID, prodID, busNumber, devNumber string) (string, string, error) {
	arg1 := fmt.Sprintf("-d%s:%s", vendorID, prodID)
	arg2 := fmt.Sprintf("-s%s:%s", busNumber, devNumber)
	b, err := runCommand(ctx, "lsusb", "-v", arg1, arg2)
	if err != nil {
		return "", "", err
	}
	lsusbOut := string(b)
	// Example output:
	//   Device Descriptor:
	//     ...
	//     idVendor           0x1d6b Linux Foundation
	//     idProduct          0x0003 3.0 root hub
	//     iManufacturer          2 Linux Foundation
	//     iProduct               3
	//     ...
	// We use these fields to get the names.
	reM := map[string]*regexp.Regexp{
		"iManufacturer": regexp.MustCompile(`^[ ]+iManufacturer[ ]+[\S]+([^\n]*)$`),
		"iProduct":      regexp.MustCompile(`^[ ]+iProduct[ ]+[\S]+([^\n]*)$`),
		"idVendor":      regexp.MustCompile(`^[ ]+idVendor[ ]+[\S]+([^\n]*)$`),
		"idProduct":     regexp.MustCompile(`^[ ]+idProduct[ ]+[\S]+([^\n]*)$`),
	}
	res := make(map[string]string)
	sc := bufio.NewScanner(strings.NewReader(lsusbOut))
	for sc.Scan() {
		for k, reg := range reM {
			m := reg.FindStringSubmatch(sc.Text())
			if m == nil {
				continue
			}
			if s := strings.Trim(m[1], " "); len(s) > 0 {
				res[k] = s
			}
		}
	}
	vendor, ok := res["idVendor"]
	if !ok {
		vendor, ok = res["iManufacturer"]
		if !ok {
			vendor = ""
		}
	}
	product, ok := res["idProduct"]
	if !ok {
		product, ok = res["iProduct"]
		if !ok {
			product = ""
		}
	}
	return vendor, product, nil
}

// matchFwupdDevice returns whether the device matches vendorID:prodID:serial.
// The matching of serial will be skipped if it is empty.
func matchFwupdDevice(device fwupdDevice, vendorID, prodID, serial string) bool {
	var matchVendor bool
	// Example arget vendor id: USB:0x1FC9.
	targetVendorID := fmt.Sprintf("USB:0x%s", strings.ToUpper(vendorID))
	if device.VendorID != nil {
		for _, vid := range strings.Split(*device.VendorID, "|") {
			if vid == targetVendorID {
				matchVendor = true
			}
		}
	}

	var matchProduct bool
	// Example target instance id: USB\VID_1FC9&PID_5002.
	targetInstanceID := fmt.Sprintf("USB\\VID_%s&PID_%s", strings.ToUpper(vendorID), strings.ToUpper(prodID))
	// Example target GUID: a01d9cb7-dc1c-52dc-88ad-ba94f473681a.
	// For the GUID generation rule in fwupd, see https://lvfs.readthedocs.io/en/latest/metainfo.html#using-guids
	targetGUID := uuid.NewSHA1(uuid.NameSpaceDNS, []byte(targetInstanceID)).String()
	for _, guid := range device.GUID {
		if guid == targetGUID {
			matchProduct = true
		}
	}

	matchSerial := serial == "" || (device.Serial != nil && serial == *device.Serial)

	return matchVendor && matchProduct && matchSerial
}

// deviceFirmwareVersion returns the firmware version info of device with
// vendorID:prodID:serial. Returns nil if no devices are matched or the matched
// devices have more than one distinct versions. The matching of serial will be
// skipped if it is empty. The version info is obtained from fwupd with its
// command line tool.
func deviceFirmwareVersion(ctx context.Context, vendorID, prodID, serial string) (*FwupdFirmwareVersionInfo, error) {
	b, err := runCommand(ctx, "fwupdmgr", "get-devices", "--show-all", "--json")
	if err != nil {
		return nil, err
	}
	// Example output: (some fields are omitted)
	// {
	//  	"Devices": [
	//  		{
	//  			"Name" : "Type-C Video Adapter",
	//  			"Guid" : [
	//  				"8964759e-69bc-5f6c-a4fa-c89c455d0228",
	//  				"a01d9cb7-dc1c-52dc-88ad-ba94f473681a"
	//  			],
	//  			"Serial" : "0000064ffcb5",
	//  			"VendorId" : "USB:0x1FC9",
	//  			"Version" : "6.45",
	//  			"VersionFormat" : "bcd",
	//  			...
	//  		},
	//  		...
	//  	]
	// }

	var fwupdResponse fwupdGetDevicesResponse
	err = json.Unmarshal(b, &fwupdResponse)
	if err != nil {
		return nil, err
	}

	var resultVersionInfo *FwupdFirmwareVersionInfo
	for _, device := range fwupdResponse.Devices {
		if matchFwupdDevice(device, vendorID, prodID, serial) {
			version := ""
			if device.Version != nil {
				version = *device.Version
			}
			versionFormat := "unknown"
			if device.VersionFormat != nil {
				versionFormat = *device.VersionFormat
			}
			versionInfo := FwupdFirmwareVersionInfo{
				Version:       version,
				VersionFormat: versionFormat,
			}
			// Returns nil when matched versions are not unique.
			if resultVersionInfo != nil && *resultVersionInfo != versionInfo {
				return nil, err
			}
			resultVersionInfo = &versionInfo
		}
	}

	if resultVersionInfo != nil && resultVersionInfo.Version != "" {
		return resultVersionInfo, err
	}
	return nil, err
}

// AttachedDevices returns attached USB devices, sorted by the fields of Device.
func AttachedDevices(ctx context.Context) ([]Device, error) {
	// Reference: https://www.kernel.org/doc/html/v4.12/driver-api/usb/usb.html#sys-kernel-debug-usb-devices-output-format

	// E.g. T:  Bus=02 Lev=00 Prnt=00 Port=00 Cnt=00 Dev#=  1 Spd=10000 MxCh= 4
	reT := regexp.MustCompile(`Bus=([0-9]{1,3}).* Dev#=\s*([0-9]{1,3})`)
	// E.g. D:  Ver= 2.00 Cls=09(hub  ) Sub=00 Prot=01 MxPS=64 #Cfgs=  1
	reD := regexp.MustCompile(`Cls=([0-9a-f]{2}).* Sub=([0-9a-f]{2}) Prot=([0-9a-f]{2})`)
	// E.g. P:  Vendor=1d6b ProdID=0002 Rev=05.04
	reP := regexp.MustCompile(`Vendor=([0-9a-f]{4}) ProdID=([0-9a-f]{4})`)
	// E.g. I:*  If#= 0 Alt= 0 #EPs= 1 Cls=09(hub  ) Sub=00 Prot=00 Driver=hub
	reI := regexp.MustCompile(`^I:[*] If#=([0-9 ]{2}) .* Cls=([0-9a-f]{2}).* Sub=([0-9a-f]{2}) Prot=([0-9a-f]{2}) Driver=([\S]*)`)
	// E.g. S:  SerialNumber=0000064ffcb5
	reSerial := regexp.MustCompile(`SerialNumber=(.*)`)

	var res []Device
	devs, err := usbDevices(ctx)
	if err != nil {
		return nil, err
	}
	for _, dev := range devs {
		var r Device
		var serial string
		var busNumber, devNumber string
		for _, line := range dev {
			switch line[0] {
			case 'T':
				m := reT.FindStringSubmatch(line)
				if m == nil {
					return nil, errors.Errorf("cannot parse usb-devices T: %v", line)
				}
				busNumber, devNumber = m[1], m[2]
			case 'D':
				m := reD.FindStringSubmatch(line)
				if m == nil {
					return nil, errors.Errorf("cannot parse usb-devices D: %v", line)
				}
				r.Class, r.SubClass, r.Protocol = m[1], m[2], m[3]
			case 'P':
				m := reP.FindStringSubmatch(line)
				if m == nil {
					return nil, errors.Errorf("cannot parse usb-devices P: %v", line)
				}
				r.VendorID, r.ProdID = m[1], m[2]
			case 'I':
				if line[2] != '*' {
					// Ignore interfaces which are not active.
					continue
				}
				m := reI.FindStringSubmatch(line)
				if m == nil {
					return nil, errors.Errorf("cannot parse usb-devices I: %v", line)
				}
				ifnum, err := strconv.ParseUint(strings.Trim(m[1], " "), 10, 8)
				if err != nil {
					return nil, errors.Wrapf(err, "cannot parse interface number %v: ", m[1])
				}
				ifc := Interface{
					InterfaceNumber: uint8(ifnum),
					Class:           m[2],
					SubClass:        m[3],
					Protocol:        m[4],
					Driver:          &m[5],
				}
				if *ifc.Driver == "(none)" {
					ifc.Driver = nil
				}
				r.Interfaces = append(r.Interfaces, ifc)
			case 'S':
				m := reSerial.FindStringSubmatch(line)
				// The matching can fail since either this device does not have
				// a serial number or this line is a descriptor of other string,
				// e.g. Manufacturer or Product.
				if m != nil {
					serial = m[1]
				}
			default:
				// It is safe to ignore other cases.
			}
		}
		var err error
		if r.VendorName, r.ProductName, err = deviceNames(ctx, r.VendorID, r.ProdID, busNumber, devNumber); err != nil {
			return nil, err
		}
		if r.FwupdFirmwareVersionInfo, err = deviceFirmwareVersion(ctx, r.VendorID, r.ProdID, serial); err != nil {
			return nil, err
		}
		res = append(res, r)
	}
	Sort(res)
	return res, nil
}

// key returns the key to sort a device. It is fields join by '$'.
func (d *Device) key() string {
	const splitter = "$"
	fields := []string{
		d.VendorID,
		d.ProdID,
		d.VendorName,
		d.ProductName,
		d.Class,
		d.SubClass,
		d.Protocol,
	}
	s := strings.Join(fields, splitter)
	for _, ifc := range d.Interfaces {
		dr := "(none)"
		if ifc.Driver != nil {
			dr = *ifc.Driver
		}
		fields = []string{
			s,
			string(ifc.InterfaceNumber),
			ifc.Class,
			ifc.SubClass,
			ifc.Protocol,
			dr,
		}
		s = strings.Join(fields, splitter)
	}
	version := "null"
	versionFormat := "null"
	if d.FwupdFirmwareVersionInfo != nil {
		version = d.FwupdFirmwareVersionInfo.Version
		versionFormat = d.FwupdFirmwareVersionInfo.VersionFormat
	}
	fields = []string{
		s,
		version,
		versionFormat,
	}
	s = strings.Join(fields, splitter)
	return s
}

// Sort sorts a slice of Devices.
func Sort(d []Device) {
	sort.Slice(d, func(i, j int) bool {
		x := d[i]
		y := d[j]
		return x.key() < y.key()
	})
}

// RemovableDevices returns the all mounted removable devices connected to DUT.
func RemovableDevices(ctx context.Context) (RemovableDeviceDetail, error) {
	var mountMap []map[string]string
	usbDevicesOut, err := testexec.CommandContext(ctx, "usb-devices").Output()
	if err != nil {
		return RemovableDeviceDetail{}, errors.Wrap(err, "failed to execute usb-devices command")
	}
	usbDevicesOutput := strings.Split(string(usbDevicesOut), "\n\n")
	var usbDevices []map[string]string
	for _, oline := range usbDevicesOutput {
		usbDevicesMap := make(map[string]string)
		for _, line := range strings.Split(oline, "\n") {
			if strings.HasPrefix(line, "D:  Ver= ") {
				usbDevicesMap["usbType"] = strings.Split(strings.Split(line, "Ver= ")[1], " ")[0]
			} else if strings.HasPrefix(line, "S:  SerialNumber=") {
				usbDevicesMap["serial"] = strings.Split(strings.Split(line, "SerialNumber=")[1], " ")[0]
			}
		}
		if !(strings.Contains(oline, "S:  SerialNumber=")) {
			usbDevicesMap["serial"] = "nil"
		}
		usbDevices = append(usbDevices, usbDevicesMap)
	}

	const mountFile = "/proc/mounts"
	fileContent, err := os.Open(mountFile)
	if err != nil {
		return RemovableDeviceDetail{}, errors.Wrap(err, "failed to open file")
	}
	defer fileContent.Close()

	// Using command 'cat /proc/mounts', we can view the status of all mounted file systems.
	// e.g.:
	/*
		...
		/dev/sda /media/removable/USB\040Drive fuseblk.ntfs rw,dirsync,nosuid...
	*/
	scanner := bufio.NewScanner(fileContent)
	mountLine := ""
	var details []MountFileDetails

	for scanner.Scan() {
		mountLine = scanner.Text()
		internalMountMap := make(map[string]string)
		// From above output checking if line contains '/media/removable/
		// then get particular device details.
		// Like e.g.
		/*
			/dev/sda as Device.
			/media/removable/USB_Drive as Mountpoint.
			fuseblk.ntfs as FsType.
			rw as Access.
			04014dfac72f63cc7bbecf5f5b as Serial
			3.20 as UsbType.
			SanDisk_3.2Gen1 as Model.
		*/
		if strings.HasPrefix(strings.Split(mountLine, " ")[1], "/media/removable/") {
			internalMountMap["device"] = strings.Split(mountLine, " ")[0]
			internalMountMap["mountpoint"] = strings.Split(mountLine, " ")[1]
			internalMountMap["fsType"] = strings.Split(mountLine, " ")[2]
			internalMountMap["access"] = strings.Split(strings.Split(mountLine, " ")[3], ",")[0]

			serialOut, err := testexec.CommandContext(ctx, "udevadm", "info", "-a", "-n", internalMountMap["device"]).Output()
			if err != nil {
				return RemovableDeviceDetail{}, errors.Wrap(err, "failed to execute udevadm command to get device serial number")
			}

			// E.g. ATTRS{serial}=="04014dfac72f63cc7bbecf5f5bd27fce201e7c4e48afa0f35f3dd
			reSerialNumber := regexp.MustCompile(`ATTRS{serial}==\"(.*)\"`)
			serialNumberMatch := reSerialNumber.FindStringSubmatch(string(serialOut))
			if len(serialNumberMatch) <= 1 {
				return RemovableDeviceDetail{}, errors.New("failed to get device serial number")
			}
			internalMountMap["serial"] = serialNumberMatch[1]

			modelOut, err := testexec.CommandContext(ctx, "udevadm", "info", "--name", internalMountMap["device"], "--query=property").Output()
			if err != nil {
				return RemovableDeviceDetail{}, errors.Wrap(err, "failed to execute udevadm command to get device model name")
			}

			// E.g. ID_MODEL=SanDisk_3.2Gen1
			reDeviceModel := regexp.MustCompile(`ID_MODEL=(.*)`)
			modelMatch := reDeviceModel.FindStringSubmatch(string(modelOut))
			if len(serialNumberMatch) <= 1 {
				return RemovableDeviceDetail{}, errors.New("failed to get device model name")
			}
			internalMountMap["model"] = modelMatch[1]

			mountMap = append(mountMap, internalMountMap)
		}
	}

	for _, item := range usbDevices {
		if item["serial"] != "nil" {
			for _, iItem := range mountMap {
				if iItem["serial"] != "nil" && iItem["serial"] == item["serial"] {
					if _, ok := iItem["usbType"]; !ok {
						details = append(details, MountFileDetails{Device: iItem["device"],
							Mountpoint: iItem["mountpoint"],
							FsType:     iItem["fsType"],
							Access:     iItem["access"],
							Serial:     iItem["serial"],
							UsbType:    item["usbType"],
							Model:      iItem["model"]})
					}
				}
			}
		}
	}
	return RemovableDeviceDetail{details}, nil
}

// MountFileDetails contains mounted device detailed info.
type MountFileDetails struct {
	// Device represents device partition like dev/sda, dev/sdb.
	Device string
	// Mountpoint represents removable device path(/media/removable/).
	Mountpoint string
	// FsType represents device file format type.
	FsType string
	// Access represents file access of USB device like read, write.
	Access string
	// Serial represents serial number of USB device.
	Serial string
	// UsbType represents USB version type like 2.0, 3.0, 3.10.
	UsbType string
	// Model represents USB brand model name.
	Model string
}

// RemovableDeviceDetail holds object for connected USB device details.
type RemovableDeviceDetail struct {
	// RemovableDevices holds connected USB devices details.
	RemovableDevices []MountFileDetails
}

// USBStorageDevicePath returns USB storage device path of provided usbDeviceType.
func USBStorageDevicePath(ctx context.Context, usbDeviceType string) (string, error) {
	var usbDevicePath string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		removableDevicesList, err := RemovableDevices(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get device list")
		}
		if len(removableDevicesList.RemovableDevices) == 0 {
			return errors.New("failed to get removable devices info")
		}
		usbType := removableDevicesList.RemovableDevices[0].UsbType
		if usbType != usbDeviceType {
			return errors.Errorf("unexpected USB version type: got %q, want %q", usbType, usbDeviceType)
		}
		usbDevicePath = removableDevicesList.RemovableDevices[0].Mountpoint
		if usbDevicePath == "" {
			return errors.New("failed to get valid devicePath")
		}
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
		return "", errors.Wrap(err, "timeout waiting to get storage path")
	}
	return usbDevicePath, nil
}
