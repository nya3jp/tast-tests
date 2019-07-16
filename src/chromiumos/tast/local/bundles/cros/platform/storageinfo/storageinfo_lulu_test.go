// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storageinfo

import (
	"reflect"
	"testing"
)

func TestParseGetStorageInfoOutputLulu(t *testing.T) {
	const out = `$ hdparm -I /dev/sda

/dev/sda:

ATA device, with non-removable media
	Model Number:       SanDisk SSD U110 16GB
	Serial Number:      145138402365
	Firmware Revision:  U221000
	Transport:          Serial, ATA8-AST, SATA Rev 2.6, SATA Rev 3.0
Standards:
	Used: unknown (minor revision code 0x0110)
	Supported: 9 8 7 6 5
	Likely used: 9
Configuration:
	Logical		max	current
	cylinders	16383	16383
	heads		16	16
	sectors/track	63	63
	--
	CHS current addressable sectors:    16514064
	LBA    user addressable sectors:    31277232
	LBA48  user addressable sectors:    31277232
	Logical  Sector size:                   512 bytes
	Physical Sector size:                   512 bytes
	Logical Sector-0 offset:                  0 bytes
	device size with M = 1024*1024:       15272 MBytes
	device size with M = 1000*1000:       16013 MBytes (16 GB)
	cache/buffer size  = unknown
	Form Factor: 1.8 inch
	Nominal Media Rotation Rate: Solid State Device
Capabilities:
	LBA, IORDY(can be disabled)
	Queue depth: 32
	Standby timer values: spec'd by Standard, no device specific minimum
	R/W multiple sector transfer: Max = 1	Current = 1
	Advanced power management level: disabled
	DMA: mdma0 mdma1 mdma2 udma0 udma1 udma2 udma3 udma4 udma5 *udma6
			Cycle time: min=120ns recommended=120ns
	PIO: pio0 pio1 pio2 pio3 pio4
			Cycle time: no flow control=120ns  IORDY flow control=120ns
Commands/features:
	Enabled	Supported:
		*	SMART feature set
			Security Mode feature set
		*	Power Management feature set
		*	Write cache
		*	Look-ahead
		*	Host Protected Area feature set
		*	WRITE_BUFFER command
		*	READ_BUFFER command
		*	NOP cmd
		*	DOWNLOAD_MICROCODE
			Advanced Power Management feature set
			SET_MAX security extension
		*	48-bit Address feature set
		*	Device Configuration Overlay feature set
		*	Mandatory FLUSH_CACHE
		*	FLUSH_CACHE_EXT
		*	SMART error logging
		*	SMART self-test
		*	General Purpose Logging feature set
		*	64-bit World wide name
		*	WRITE_UNCORRECTABLE_EXT command
		*	Segmented DOWNLOAD_MICROCODE
		*	Gen1 signaling speed (1.5Gb/s)
		*	Gen2 signaling speed (3.0Gb/s)
		*	Gen3 signaling speed (6.0Gb/s)
		*	Native Command Queueing (NCQ)
		*	Phy event counters
		*	Device-initiated interface power management
		*	Software settings preservation
			Device Sleep (DEVSLP)
		*	SANITIZE feature set
		*	BLOCK_ERASE_EXT command
		*	SET MAX SETPASSWORD/UNLOCK DMA commands
		*	DEVICE CONFIGURATION SET/IDENTIFY DMA commands
		*	Data Set Management TRIM supported (limit 8 blocks)
		*	Deterministic read data after TRIM
Security:
	Master password revision code = 65534
		supported
	not	enabled
	not	locked
	not	frozen
	not	expired: security count
		supported: enhanced erase
	2min for SECURITY ERASE UNIT. 2min for ENHANCED SECURITY ERASE UNIT.
Logical Unit WWN Device Identifier: 5001b44cd681403d
	NAA		: 5
	IEEE OUI	: 001b44
	Unique ID	: cd681403d
Checksum: correct

$ smartctl -x /dev/sda
smartctl 6.6 2017-11-05 r4594 [x86_64-linux-3.14.0] (local build)
Copyright (C) 2002-17, Bruce Allen, Christian Franke, www.smartmontools.org

=== START OF INFORMATION SECTION ===
Model Family:     SanDisk based SSDs
Device Model:     SanDisk SSD U110 16GB
Serial Number:    145138402365
LU WWN Device Id: 5 001b44 cd681403d
Firmware Version: U221000
User Capacity:    16,013,942,784 bytes [16.0 GB]
Sector Size:      512 bytes logical/physical
Rotation Rate:    Solid State Device
Form Factor:      1.8 inches
Device is:        In smartctl database [for details use: -P show]
ATA Version is:   ACS-2 T13/2015-D revision 3
SATA Version is:  SATA 3.0, 6.0 Gb/s (current: 6.0 Gb/s)
Local Time is:    Tue Jul 30 21:28:57 2019 MST
SMART support is: Available - device has SMART capability.
SMART support is: Enabled
AAM feature is:   Unavailable
APM feature is:   Disabled
Rd look-ahead is: Enabled
Write cache is:   Enabled
DSN feature is:   Unavailable
ATA Security is:  Disabled, NOT FROZEN [SEC1]
Wt Cache Reorder: Unavailable

=== START OF READ SMART DATA SECTION ===
SMART overall-health self-assessment test result: PASSED

General SMART Values:
Offline data collection status:  (0x00)	Offline data collection activity
					was never started.
					Auto Offline Data Collection: Disabled.
Self-test execution status:      (   0)	The previous self-test routine completed
					without error or no self-test has ever
					been run.
Total time to complete Offline
data collection: 		(  120) seconds.
Offline data collection
capabilities: 			 (0x51) SMART execute Offline immediate.
					No Auto Offline data collection support.
					Suspend Offline collection upon new
					command.
					No Offline surface scan supported.
					Self-test supported.
					No Conveyance Self-test supported.
					Selective Self-test supported.
SMART capabilities:            (0x0003)	Saves SMART data before entering
					power-saving mode.
					Supports SMART auto save timer.
Error logging capability:        (0x01)	Error logging supported.
					General Purpose Logging supported.
Short self-test routine
recommended polling time: 	 (   2) minutes.
Extended self-test routine
recommended polling time: 	 (   3) minutes.

SMART Attributes Data Structure revision number: 1
Vendor Specific SMART Attributes with Thresholds:
ID# ATTRIBUTE_NAME          FLAGS    VALUE WORST THRESH FAIL RAW_VALUE
	5 Reallocated_Sector_Ct   -O----   100   100   000    -    0
	9 Power_On_Hours          -O----   100   100   000    -    333
	12 Power_Cycle_Count       -O----   100   100   000    -    1758
165 Total_Write/Erase_Count -O----   100   100   000    -    174113
171 Program_Fail_Count      -O----   100   100   000    -    0
172 Erase_Fail_Count        -O----   100   100   000    -    0
173 Avg_Write/Erase_Count   -O----   100   100   000    -    86
174 Unexpect_Power_Loss_Ct  -O----   100   100   000    -    17
187 Reported_Uncorrect      -O----   100   100   000    -    0
194 Temperature_Celsius     -O---K   055   045   000    -    45 (Min/Max 14/57)
230 Perc_Write/Erase_Count  -O----   100   100   000    -    286
232 Perc_Avail_Resrvd_Space PO----   100   100   005    -    0
234 Perc_Write/Erase_Ct_BC  -O----   100   100   000    -    252
241 Total_LBAs_Written      -O----   100   100   000    -    2117789970
242 Total_LBAs_Read         -O----   100   100   000    -    1714398091
							||||||_ K auto-keep
							|||||__ C event count
							||||___ R error rate
							|||____ S speed/performance
							||_____ O updated online
							|______ P prefailure warning

General Purpose Log Directory Version 1
SMART           Log Directory Version 1 [multi-sector log support]
Address    Access  R/W   Size  Description
0x00       GPL,SL  R/O      1  Log Directory
0x01       GPL,SL  R/O      1  Summary SMART error log
0x03       GPL,SL  R/O     16  Ext. Comprehensive SMART error log
0x04       GPL,SL  R/O      8  Device Statistics log
0x06       GPL,SL  R/O      1  SMART self-test log
0x09       GPL,SL  R/W      1  Selective self-test log
0x10       GPL,SL  R/O      1  NCQ Command Error log
0x11       GPL,SL  R/O      1  SATA Phy Event Counters log
0x30       GPL,SL  R/O      9  IDENTIFY DEVICE data log
0x80-0x9f  GPL,SL  R/W     16  Host vendor specific log
0xa1       GPL,SL  VS       1  Device vendor specific log
0xa2       GPL,SL  VS       2  Device vendor specific log
0xa3       GPL,SL  VS       1  Device vendor specific log
0xa6-0xa7  GPL,SL  VS     255  Device vendor specific log

Warning! SMART Extended Comprehensive Error Log Structure error: invalid SMART checksum.
SMART Extended Comprehensive Error Log Version: 1 (16 sectors)
No Errors Logged

SMART Extended Self-test Log (GP Log 0x07) not supported

SMART Self-test log structure revision number 1
No self-tests have been logged.  [To run self-tests, use: smartctl -t]

SMART Selective self-test log data structure revision number 1
	SPAN  MIN_LBA  MAX_LBA  CURRENT_TEST_STATUS
	1        0        0  Not_testing
	2        0        0  Not_testing
	3        0        0  Not_testing
	4        0        0  Not_testing
	5        0        0  Not_testing
Selective self-test flags (0x0):
	After scanning selected spans, do NOT read-scan remainder of disk.
If Selective self-test is pending on power-up, resume after 0 minute delay.

SCT Commands not supported

Device Statistics (GP Log 0x04)
Page  Offset Size        Value Flags Description
0x05  =====  =               =  ===  == Temperature Statistics (rev 1) ==
0x05  0x008  1              45  ---  Current Temperature
0x05  0x010  1               -  ---  Average Short Term Temperature
0x05  0x018  1               -  ---  Average Long Term Temperature
0x05  0x020  1              57  ---  Highest Temperature
0x05  0x028  1              26  ---  Lowest Temperature
0x05  0x030  1               -  ---  Highest Average Short Term Temperature
0x05  0x038  1               -  ---  Lowest Average Short Term Temperature
0x05  0x040  1               -  ---  Highest Average Long Term Temperature
0x05  0x048  1               -  ---  Lowest Average Long Term Temperature
0x05  0x050  4               0  ---  Time in Over-Temperature
0x05  0x058  1              95  ---  Specified Maximum Operating Temperature
0x05  0x060  4               0  ---  Time in Under-Temperature
0x05  0x068  1               0  ---  Specified Minimum Operating Temperature
0x07  =====  =               =  ===  == Solid State Device Statistics (rev 1) ==
0x07  0x008  1               2  N--  Percentage Used Endurance Indicator
								|||_ C monitored condition met
								||__ D supports DSN
								|___ N normalized value

Pending Defects log (GP Log 0x0c) not supported

SATA Phy Event Counters (GP Log 0x11)
ID      Size     Value  Description
0x0003  2            0  R_ERR response for device-to-host data FIS
0x0004  2            0  R_ERR response for host-to-device data FIS
0x0006  2            0  R_ERR response for device-to-host non-data FIS
0x0007  2            0  R_ERR response for host-to-device non-data FIS
0x0009  2          461  Transition from drive PhyRdy to drive PhyNRdy
0x000a  2            8  Device-to-host register FISes sent due to a COMRESET
0x000f  2            0  R_ERR response for host-to-device data FIS, CRC
0x0012  2            0  R_ERR response for host-to-device non-data FIS, CRC
0x0001  2            0  Command failed due to ICRC error
`

	info, err := parseGetStorageInfoOutput([]byte(out))
	if err != nil {
		t.Fatal("parseGetStorageInfoOutput() failed: ", err)
	}

	exp := &Info{
		Type:   SSD,
		Status: Healthy,
	}

	if !reflect.DeepEqual(info, exp) {
		t.Errorf("parseGetStorageInfoOutput() = %+v; want %+v", info, exp)
	}
}
