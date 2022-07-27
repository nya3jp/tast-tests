// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package usbutil

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/errors"
)

var files = map[string]string{
	"/sys/kernel/debug/usb/devices": `
T:  Bus=02 Lev=00 Prnt=00 Port=00 Cnt=00 Dev#=  1 Spd=10000 MxCh= 4
D:  Ver= 3.10 Cls=09(hub  ) Sub=00 Prot=03 MxPS= 9 #Cfgs=  1
P:  Vendor=1a2b ProdID=3c4d Rev=05.04
S:  Manufacturer=Linux 1.2.3-abcdefg64 xhci-hcd
S:  Product=xHCI Host Controller
S:  SerialNumber=0000:04:00.3
C:* #Ifs= 1 Cfg#= 1 Atr=e0 MxPwr=0mA
I:* If#= 0 Alt= 0 #EPs= 1 Cls=09(hub  ) Sub=00 Prot=00 Driver=hub
C:  #Ifs= 1 Cfg#= 2 Atr=e0 MxPwr=0mA
I:  If#= 0 Alt= 0 #EPs= 1 Cls=09(hub  ) Sub=01 Prot=01 Driver=hub

T:  Bus=02 Lev=00 Prnt=00 Port=00 Cnt=00 Dev#=  1 Spd=10000 MxCh= 4
D:  Ver= 3.10 Cls=09(hub  ) Sub=00 Prot=03 MxPS= 9 #Cfgs=  1
P:  Vendor=1a2b ProdID=3c4e Rev=05.04
S:  Manufacturer=Linux 1.2.3-abcdefg64 xhci-hcd
S:  Product=xHCI Host Controller
S:  SerialNumber=0000:04:00.3
C:* #Ifs= 1 Cfg#= 1 Atr=e0 MxPwr=0mA
I:* If#= 0 Alt= 0 #EPs= 1 Cls=09(hub  ) Sub=00 Prot=00 Driver=hub
C:  #Ifs= 1 Cfg#= 2 Atr=e0 MxPwr=0mA
I:  If#= 0 Alt= 0 #EPs= 1 Cls=09(hub  ) Sub=01 Prot=01 Driver=hub

T:  Bus=01 Lev=01 Prnt=01 Port=04 Cnt=01 Dev#=  2 Spd=480 MxCh= 0 
D:  Ver= 2.01 Cls=ef(misc ) Sub=02 Prot=01 MxPS=64 #Cfgs=  1
P:  Vendor=1a2b ProdID=5e6f Rev=00.02
S:  Manufacturer=Alice 
S:  Product=USB2.0 HD UVC WebCam
S:  SerialNumber=0x0001                                   
C:* #Ifs= 3 Cfg#= 1 Atr=80 MxPwr=500mA
I:* If#= 0 Alt= 0 #EPs= 1 Cls=0e(video) Sub=01 Prot=00 Driver=uvcvideo
I:* If#=01 Alt= 0 #EPs= 0 Cls=0e(video) Sub=02 Prot=00 Driver=uvcvideo
I:* If#= 2 Alt= 0 #EPs= 0 Cls=fe(app. ) Sub=01 Prot=01 Driver=(none)
C:  #Ifs= 1 Cfg#= 2 Atr=e0 MxPwr=0mA
I:  If#= 0 Alt= 0 #EPs= 1 Cls=09(hub  ) Sub=01 Prot=01 Driver=hub

T:  Bus=03 Lev=01 Prnt=01 Port=08 Cnt=03 Dev#=  4 Spd=12   MxCh= 0
D:  Ver= 2.01 Cls=00(>ifc ) Sub=00 Prot=00 MxPS=64 #Cfgs=  1
P:  Vendor=1fc9 ProdID=5002 Rev= 6.45
S:  Manufacturer=NXP
S:  Product=Type-C Video Adapter
S:  SerialNumber=0000074f7cb5
C:* #Ifs= 1 Cfg#= 1 Atr=80 MxPwr=100mA
I:* If#= 0 Alt= 0 #EPs= 0 Cls=fe(app. ) Sub=01 Prot=01 Driver=(none)

T:  Bus=04 Lev=02 Prnt=22 Port=00 Cnt=01 Dev#= 1 Spd=480  MxCh= 0
D:  Ver= 2.10 Cls=00(>ifc ) Sub=00 Prot=00 MxPS=64 #Cfgs=  2
P:  Vendor=0bda ProdID=8153 Rev=31.00
S:  Manufacturer=Realtek
S:  Product=USB 10/100/1000 LAN
S:  SerialNumber=001000001
C:* #Ifs= 1 Cfg#= 1 Atr=a0 MxPwr=350mA
I:* If#= 0 Alt= 0 #EPs= 3 Cls=ff(vend.) Sub=ff Prot=00 Driver=r8152

T:  Bus=05 Lev=01 Prnt=01 Port=00 Cnt=01 Dev#=  2 Spd=5000 MxCh= 0
D:  Ver= 3.00 Cls=00(>ifc ) Sub=00 Prot=00 MxPS= 9 #Cfgs=  2
P:  Vendor=0bda ProdID=8153 Rev=31.00
S:  Manufacturer=UNITEK
S:  Product=UNITEK Y-3470B
S:  SerialNumber=001000001
C:* #Ifs= 1 Cfg#= 1 Atr=a0 MxPwr=288mA
I:* If#= 0 Alt= 0 #EPs= 3 Cls=ff(vend.) Sub=ff Prot=00 Driver=r8152
`,
}

var cmdRes = map[string]string{
	"lsusb -v -d1a2b:3c4d -s02:1": `Bus 002 Device 001: ID 1d6b:0003 Linux Foundation 3.0 root hub
Couldn't open device, some information will be missing
Device Descriptor:                                        
  bLength                18                               
  bDescriptorType         1                               
  bcdUSB               3.00                               
  bDeviceClass            9 Hub    
  bDeviceSubClass         0                               
  bDeviceProtocol         3                               
  bMaxPacketSize0         9                               
  idVendor           0x1d6b Linux Foundation
  idProduct          0x0003 3.0 root hub
  bcdDevice            5.10                               
  iManufacturer           3
  iProduct                2                               
  iSerial                 1                               
  bNumConfigurations      1                               
`,
	"lsusb -v -d1a2b:3c4e -s02:1": `Bus 002 Device 001: ID 1d6b:0003 Linux Foundation 3.0 root hub
Couldn't open device, some information will be missing
Device Descriptor:                                        
  bLength                18                               
  bDescriptorType         1                               
  bcdUSB               3.00                               
  bDeviceClass            9 Hub    
  bDeviceSubClass         0                               
  bDeviceProtocol         3                               
  bMaxPacketSize0         9                               
  idVendor           0x1d6b Linux Foundation
  idProduct          0x0003
  bcdDevice            5.10                               
  iManufacturer           3 Shouldn't be used
  iProduct                2                               
  iSerial                 1                               
  bNumConfigurations      1                               
`,
	// There could be multiple same devices.
	"lsusb -v -d1a2b:5e6f -s01:2": `Bus 001 Device 002: ID 1a2b:5e6f Alice, Inc. USB2.0 HD UVC WebCam
Couldn't open device, some information will be missing
Device Descriptor:                                        
  bLength                18                               
  bDescriptorType         1                               
  bcdUSB               3.00                               
  bDeviceClass            9 Hub    
  bDeviceSubClass         0                               
  bDeviceProtocol         3                               
  bMaxPacketSize0         9                               
  idVendor           0x1d6b Alice, Inc.
  idProduct          0x0003 
  bcdDevice            5.10                               
  iManufacturer           3 Alice, Inc.
  iProduct                2   USB2.0 HD UVC WebCam 
  bNumConfigurations      1                               

Bus 001 Device 002: ID 1a2b:5e6f Alice, Inc. USB2.0 HD UVC WebCam
Couldn't open device, some information will be missing
Device Descriptor:                                        
  bLength                18                               
  bDescriptorType         1                               
  bcdUSB               3.00                               
  bDeviceClass            9 Hub    
  bDeviceSubClass         0                               
  bDeviceProtocol         3                               
  bMaxPacketSize0         9                               
  idVendor           0x1d6b Alice, Inc.
  idProduct          0x0003 
  bcdDevice            5.10                               
  iManufacturer           3 Alice, Inc. Shouldn't be used
  iProduct                2   USB2.0 HD UVC WebCam 
  iSerial                 1                               
  bNumConfigurations      1                               
`,
	// Two devices with the same idVendor:idProduct can have different iProduct strings.
	// Use busNumber:devNumber to tell them apart.
	"lsusb -v -d0bda:8153 -s04:1": `Bus 004 Device 001: ID 0bda:8153 Realtek Semiconductor Corp. UNITEK Y-3470B
Device Descriptor:
  bLength                18
  bDescriptorType         1
  bcdUSB               3.00
  bDeviceClass            0 
  bDeviceSubClass         0 
  bDeviceProtocol         0 
  bMaxPacketSize0         9
  idVendor           0x0bda Realtek Semiconductor Corp.
  idProduct          0x8153 
  bcdDevice           31.00
  iManufacturer           1 UNITEK
  iProduct                2 UNITEK Y-3470B
  iSerial                 6 001000001
  bNumConfigurations      2
`,
	"lsusb -v -d0bda:8153 -s05:2": `
Bus 005 Device 002: ID 0bda:8153 Realtek Semiconductor Corp. USB 10/100/1000 LAN
Device Descriptor:
  bLength                18
  bDescriptorType         1
  bcdUSB               2.10
  bDeviceClass            0 
  bDeviceSubClass         0 
  bDeviceProtocol         0 
  bMaxPacketSize0        64
  idVendor           0x0bda Realtek Semiconductor Corp.
  idProduct          0x8153 
  bcdDevice           31.00
  iManufacturer           1 Realtek
  iProduct                2 USB 10/100/1000 LAN
  iSerial                 6 001000001
  bNumConfigurations      2
`,
	"lsusb -v -d1fc9:5002 -s03:4": `Bus 003 Device 004: ID 1fc9:5002 NXP Semiconductors Type-C Video Adapter
Device Descriptor:
  bLength                18
  bDescriptorType         1
  bcdUSB               2.01
  bDeviceClass            0
  bDeviceSubClass         0
  bDeviceProtocol         0
  bMaxPacketSize0        64
  idVendor           0x1fc9 NXP Semiconductors
  idProduct          0x5002
  bcdDevice            6.45
  iManufacturer           1 NXP
  iProduct                2 Type-C Video Adapter
  iSerial                 3 0000074f7cb5
  bNumConfigurations      1
`,
	"fwupdmgr get-devices --show-all --json": `{
  "Devices": [
    {
      "Name" : "Type-C Video Adapter",
      "Guid" : [
        "8964759e-69bc-5f6c-a4fa-c89c455d0228",
        "a01d9cb7-dc1c-52dc-88ad-ba94f473681a"
      ],
      "Serial" : "0000074f7cb5",
      "VendorId" : "USB:0x1FC9",
      "Version" : "6.45",
      "VersionFormat" : "bcd"
    }
  ]
}`,
}

func ptr(s string) *string {
	return &s
}

func TestGetDevices(t *testing.T) {
	readFile = func(fpath string) ([]byte, error) {
		s, ok := files[fpath]
		if !ok {
			return nil, errors.Errorf("unexpected file: %v", fpath)
		}
		return []byte(s), nil
	}
	runCommand = func(ctx context.Context, cmd string, args ...string) ([]byte, error) {
		args = append([]string{cmd}, args...)
		s, ok := cmdRes[strings.Join(args, " ")]
		if !ok {
			return nil, errors.Errorf("unexpected arguments: %v", args)
		}
		return []byte(s), nil
	}

	g, err := GetDevices(context.Background())
	if err != nil {
		t.Fatal("Failed to run GetDevices:", err)
	}
	e := []Device{
		Device{
			VendorID:    "0bda",
			ProdID:      "8153",
			VendorName:  "Realtek Semiconductor Corp.",
			ProductName: "UNITEK Y-3470B",
			Class:       "00",
			SubClass:    "00",
			Protocol:    "00",
			Interfaces: []Interface{
				Interface{
					InterfaceNumber: 0,
					Class:           "ff",
					SubClass:        "ff",
					Protocol:        "00",
					Driver:          ptr("r8152"),
				},
			},
		},
		Device{
			VendorID:    "0bda",
			ProdID:      "8153",
			VendorName:  "Realtek Semiconductor Corp.",
			ProductName: "USB 10/100/1000 LAN",
			Class:       "00",
			SubClass:    "00",
			Protocol:    "00",
			Interfaces: []Interface{
				Interface{
					InterfaceNumber: 0,
					Class:           "ff",
					SubClass:        "ff",
					Protocol:        "00",
					Driver:          ptr("r8152"),
				},
			},
		},
		Device{
			VendorID:    "1a2b",
			ProdID:      "3c4d",
			VendorName:  "Linux Foundation",
			ProductName: "3.0 root hub",
			Class:       "09",
			SubClass:    "00",
			Protocol:    "03",
			Interfaces: []Interface{
				Interface{
					InterfaceNumber: 0,
					Class:           "09",
					SubClass:        "00",
					Protocol:        "00",
					Driver:          ptr("hub"),
				},
			},
		},
		Device{
			VendorID:    "1a2b",
			ProdID:      "3c4e",
			VendorName:  "Linux Foundation",
			ProductName: "",
			Class:       "09",
			SubClass:    "00",
			Protocol:    "03",
			Interfaces: []Interface{
				Interface{
					InterfaceNumber: 0,
					Class:           "09",
					SubClass:        "00",
					Protocol:        "00",
					Driver:          ptr("hub"),
				},
			},
		},
		Device{
			VendorID:    "1a2b",
			ProdID:      "5e6f",
			VendorName:  "Alice, Inc.",
			ProductName: "USB2.0 HD UVC WebCam",
			Class:       "ef",
			SubClass:    "02",
			Protocol:    "01",
			Interfaces: []Interface{
				Interface{
					InterfaceNumber: 0,
					Class:           "0e",
					SubClass:        "01",
					Protocol:        "00",
					Driver:          ptr("uvcvideo"),
				},
				Interface{
					InterfaceNumber: 1,
					Class:           "0e",
					SubClass:        "02",
					Protocol:        "00",
					Driver:          ptr("uvcvideo"),
				},
				Interface{
					InterfaceNumber: 2,
					Class:           "fe",
					SubClass:        "01",
					Protocol:        "01",
					Driver:          nil,
				},
			},
		},
		Device{
			VendorID:    "1fc9",
			ProdID:      "5002",
			VendorName:  "NXP Semiconductors",
			ProductName: "Type-C Video Adapter",
			Class:       "00",
			SubClass:    "00",
			Protocol:    "00",
			Interfaces: []Interface{
				Interface{
					InterfaceNumber: 0,
					Class:           "fe",
					SubClass:        "01",
					Protocol:        "01",
					Driver:          nil,
				},
			},
			FwupdFirmwareVersionInfo: &FwupdFirmwareVersionInfo{
				Version:       "6.45",
				VersionFormat: "bcd",
			},
		},
	}
	if d := cmp.Diff(e, g); d != "" {
		t.Fatal("USB test failed (-expected + got): ", d)
	}
}
