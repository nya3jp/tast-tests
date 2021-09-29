// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package usb

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/errors"
)

var cmdRes = map[string]string{
	"usb-devices": `
T:  Bus=02 Lev=00 Prnt=00 Port=00 Cnt=00 Dev#=  1 Spd=10000 MxCh= 4
D:  Ver= 3.10 Cls=09(hub  ) Sub=00 Prot=03 MxPS= 9 #Cfgs=  1
P:  Vendor=1a2b ProdID=3c4d Rev=05.04
S:  Manufacturer=Linux 1.2.3-abcdefg64 xhci-hcd
S:  Product=xHCI Host Controller
S:  SerialNumber=0000:04:00.3
C:  #Ifs= 1 Cfg#= 1 Atr=e0 MxPwr=0mA
I:  If#=0x0 Alt= 0 #EPs= 1 Cls=09(hub  ) Sub=00 Prot=00 Driver=hub

T:  Bus=02 Lev=00 Prnt=00 Port=00 Cnt=00 Dev#=  1 Spd=10000 MxCh= 4
D:  Ver= 3.10 Cls=09(hub  ) Sub=00 Prot=03 MxPS= 9 #Cfgs=  1
P:  Vendor=1a2b ProdID=3c4e Rev=05.04
S:  Manufacturer=Linux 1.2.3-abcdefg64 xhci-hcd
S:  Product=xHCI Host Controller
S:  SerialNumber=0000:04:00.3
C:  #Ifs= 1 Cfg#= 1 Atr=e0 MxPwr=0mA
I:  If#=0x0 Alt= 0 #EPs= 1 Cls=09(hub  ) Sub=00 Prot=00 Driver=hub

T:  Bus=01 Lev=01 Prnt=01 Port=04 Cnt=01 Dev#=  2 Spd=480 MxCh= 0 
D:  Ver= 2.01 Cls=ef(misc ) Sub=02 Prot=01 MxPS=64 #Cfgs=  1
P:  Vendor=1a2b ProdID=5e6f Rev=00.02
S:  Manufacturer=Alice 
S:  Product=USB2.0 HD UVC WebCam
S:  SerialNumber=0x0001                                   
C:  #Ifs= 3 Cfg#= 1 Atr=80 MxPwr=500mA
I:  If#=0x0 Alt= 0 #EPs= 1 Cls=0e(video) Sub=01 Prot=00 Driver=uvcvideo
I:  If#=0x1 Alt= 0 #EPs= 0 Cls=0e(video) Sub=02 Prot=00 Driver=uvcvideo
I:  If#=0x2 Alt= 0 #EPs= 0 Cls=fe(app. ) Sub=01 Prot=01 Driver=(none)
`,
	"lsusb -v -d1a2b:3c4d": `Bus 002 Device 001: ID 1d6b:0003 Linux Foundation 3.0 root hub
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
	"lsusb -v -d1a2b:3c4e": `Bus 002 Device 001: ID 1d6b:0003 Linux Foundation 3.0 root hub
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
	"lsusb -v -d1a2b:5e6f": `Bus 002 Device 002: ID 1a2b:5e6f Alice, Inc. USB2.0 HD UVC WebCam
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

Bus 002 Device 002: ID 1a2b:5e6f Alice, Inc. USB2.0 HD UVC WebCam
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
}

func ptr(s string) *string {
	return &s
}

func TestExpectedDevices(t *testing.T) {
	runCommand = func(ctx context.Context, cmd string, args ...string) ([]byte, error) {
		args = append([]string{cmd}, args...)
		s, ok := cmdRes[strings.Join(args, " ")]
		if !ok {
			return nil, errors.Errorf("unexpected arguments: %v", args)
		}
		return []byte(s), nil
	}

	g, err := ExpectedDevices(context.Background())
	if err != nil {
		t.Fatal("Failed to run ExpectedDevices:", err)
	}
	e := []Device{
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
					InterfaceNumber: "0",
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
					InterfaceNumber: "0",
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
					InterfaceNumber: "0",
					Class:           "0e",
					SubClass:        "01",
					Protocol:        "00",
					Driver:          ptr("uvcvideo"),
				},
				Interface{
					InterfaceNumber: "1",
					Class:           "0e",
					SubClass:        "02",
					Protocol:        "00",
					Driver:          ptr("uvcvideo"),
				},
				Interface{
					InterfaceNumber: "2",
					Class:           "fe",
					SubClass:        "01",
					Protocol:        "01",
					Driver:          nil,
				},
			},
		},
	}
	if d := cmp.Diff(e, g); d != "" {
		t.Fatal("USB test failed (-expected + got): ", d)
	}
}
