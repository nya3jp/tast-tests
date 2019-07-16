// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storageinfo

import (
	"reflect"
	"testing"
)

func TestParseGetStorageInfoOutputEve(t *testing.T) {
	const out = `$ smartctl -x /dev/nvme0
smartctl 6.6 2017-11-05 r4594 [x86_64-linux-4.4.185] (local build)
Copyright (C) 2002-17, Bruce Allen, Christian Franke, www.smartmontools.org


=== START OF INFORMATION SECTION ===
Model Number:                           SAMSUNG KUS040205M-B001
Serial Number:                          S3VBNY0J708174
Firmware Version:                       DXC83G1Q
PCI Vendor/Subsystem ID:                0x144d
IEEE OUI Identifier:                    0x002538
Total NVM Capacity:                     512,110,190,592 [512 GB]
Unallocated NVM Capacity:               0
Controller ID:                          3
Number of Namespaces:                   1
Namespace 1 Size/Capacity:              512,110,190,592 [512 GB]
Namespace 1 Utilization:                23,092,166,656 [23.0 GB]
Namespace 1 Formatted LBA Size:         512
Local Time is:                          Thu Aug  1 11:06:45 2019 PDT
Firmware Updates (0x16):                3 Slots, no Reset required
Optional Admin Commands (0x0006):   Format Frmw_DL
Optional NVM Commands (0x001f):         Comp Wr_Unc DS_Mngmt Wr_Zero Sav/Sel_Feat
Maximum Data Transfer Size:             128 Pages
Warning  Comp. Temp. Threshold:         91 Celsius
Critical Comp. Temp. Threshold:         93 Celsius


Supported Power States
St Op         Max   Active         Idle   RL RT WL WT  Ent_Lat  Ex_Lat
	0 +         3.00W           -            -        0  0  0  0            0           0
	1 +         2.40W           -            -        1  1  1  1            5           5
	2 +         1.90W           -            -        2  2  2  2           10          10
	3 -   0.0600W           -            -        3  3  3  3          300         800
	4 -   0.0050W           -            -        4  4  4  4         1800        3700


Supported LBA Sizes (NSID 0x1)
Id Fmt  Data  Metadt  Rel_Perf
	0 +         512           0             0
	1 -        4096           0             0


=== START OF SMART DATA SECTION ===
SMART overall-health self-assessment test result: PASSED


SMART/Health Information (NVMe Log 0x02, NSID 0xffffffff)
Critical Warning:                       0x00
Temperature:                            36 Celsius
Available Spare:                        100%
Available Spare Threshold:              10%
Percentage Used:                        0%
Data Units Read:                        31,116,214 [15.9 TB]
Data Units Written:                     34,486,615 [17.6 TB]
Host Read Commands:                     41,112,741
Host Write Commands:                    89,735,226
Controller Busy Time:                   1,233
Power Cycles:                           321
Power On Hours:                         245
Unsafe Shutdowns:                       174
Media and Data Integrity Errors:        0
Error Information Log Entries:          0
Warning  Comp. Temperature Time:        0
Critical Comp. Temperature Time:        0
Temperature Sensor 1:                   36 Celsius


Error Information (NVMe Log 0x01, max 64 entries)
No Errors Logged
`

	info, err := parseGetStorageInfoOutput([]byte(out))
	if err != nil {
		t.Fatal("parseGetStorageInfoOutput() failed: ", err)
	}

	exp := &Info{
		Type:   NVMe,
		Status: Healthy,
	}

	if !reflect.DeepEqual(info, exp) {
		t.Errorf("parseGetStorageInfoOutput() = %+v; want %+v", info, exp)
	}
}
