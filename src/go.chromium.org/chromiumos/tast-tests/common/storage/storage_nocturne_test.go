// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storage

import (
	"context"
	"reflect"
	"testing"
)

func TestParseGetStorageInfoOutputNocturne(t *testing.T) {
	const out = `cid                  | 90014a684445615033011137e0d58500
csd                  | d02700328f5903ffffffffef8e400000
date                 | 08/2018
enhanced_area_offset | 18446744073709551594
enhanced_area_size   | 4294967274
erase_size           | 524288
fwrev                | 0x3030303300000000
hwrev                | 0x0
manfid               | 0x000090
name                 | hDEaP3
oemid                | 0x014a
preferred_erase_size | 4194304
prv                  | 0x1
raw_rpmb_size_mult   | 0x80
rel_sectors          | 0x1
serial               | 0x1137e0d5
=============================================
	Extended CSD rev 1.8 (MMC 5.1)
=============================================

Card Supported Command sets [S_CMD_SET: 0x01]
HPI Features [HPI_FEATURE: 0x01]: implementation based on CMD13
Background operations support [BKOPS_SUPPORT: 0x01]
Max Packet Read Cmd [MAX_PACKED_READS: 0x3f]
Max Packet Write Cmd [MAX_PACKED_WRITES: 0x3f]
Data TAG support [DATA_TAG_SUPPORT: 0x01]
Data TAG Unit Size [TAG_UNIT_SIZE: 0x00]
Tag Resources Size [TAG_RES_SIZE: 0x00]
Context Management Capabilities [CONTEXT_CAPABILITIES: 0x05]
Large Unit Size [LARGE_UNIT_SIZE_M1: 0x01]
Extended partition attribute support [EXT_SUPPORT: 0x03]
Supported modes [SUPPORTED_MODES: 0x01]
FFU features [FFU_FEATURES: 0x00]
Operation codes timeout [OPERATION_CODE_TIMEOUT: 0x00]
FFU Argument [FFU_ARG: 0xfffafff0]
Number of FW sectors correctly programmed [NUMBER_OF_FW_SECTORS_CORRECTLY_PROGRAMMED: 0]
Vendor proprietary health report:
[VENDOR_PROPRIETARY_HEALTH_REPORT[301]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[300]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[299]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[298]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[297]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[296]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[295]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[294]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[293]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[292]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[291]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[290]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[289]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[288]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[287]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[286]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[285]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[284]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[283]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[282]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[281]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[280]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[279]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[278]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[277]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[276]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[275]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[274]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[273]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[272]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[271]]: 0x00
[VENDOR_PROPRIETARY_HEALTH_REPORT[270]]: 0x00
Device life time estimation type B [DEVICE_LIFE_TIME_EST_TYP_B: 0x01]
	i.e. 0% - 10% device life time used
Device life time estimation type A [DEVICE_LIFE_TIME_EST_TYP_A: 0x00]
Pre EOL information [PRE_EOL_INFO: 0x01]
	i.e. Normal
Optimal read size [OPTIMAL_READ_SIZE: 0x20]
Optimal write size [OPTIMAL_WRITE_SIZE: 0x20]
Optimal trim unit size [OPTIMAL_TRIM_UNIT_SIZE: 0x01]
Device version [DEVICE_VERSION: 0x44 - 0x03]
Firmware version:
[FIRMWARE_VERSION[261]]: 0x00
[FIRMWARE_VERSION[260]]: 0x00
[FIRMWARE_VERSION[259]]: 0x00
[FIRMWARE_VERSION[258]]: 0x00
[FIRMWARE_VERSION[257]]: 0x33
[FIRMWARE_VERSION[256]]: 0x30
[FIRMWARE_VERSION[255]]: 0x30
[FIRMWARE_VERSION[254]]: 0x30
Power class for 200MHz, DDR at VCC= 3.6V [PWR_CL_DDR_200_360: 0x04]
Generic CMD6 Timer [GENERIC_CMD6_TIME: 0x0a]
Power off notification [POWER_OFF_LONG_TIME: 0x3c]
Cache Size [CACHE_SIZE] is 2048 KiB
Background operations status [BKOPS_STATUS: 0x00]
1st Initialisation Time after programmed sector [INI_TIMEOUT_AP: 0x0a]
Power class for 52MHz, DDR at 3.6V [PWR_CL_DDR_52_360: 0x00]
Power class for 52MHz, DDR at 1.95V [PWR_CL_DDR_52_195: 0x00]
Power class for 200MHz at 3.6V [PWR_CL_200_360: 0x00]
Power class for 200MHz, at 1.95V [PWR_CL_200_195: 0x00]
Minimum Performance for 8bit at 52MHz in DDR mode:
	[MIN_PERF_DDR_W_8_52: 0x00]
	[MIN_PERF_DDR_R_8_52: 0x00]
TRIM Multiplier [TRIM_MULT: 0x02]
Secure Feature support [SEC_FEATURE_SUPPORT: 0x55]
Boot Information [BOOT_INFO: 0x07]
	Device supports alternative boot method
	Device supports dual data rate during boot
	Device supports high speed timing during boot
Boot partition size [BOOT_SIZE_MULTI: 0x20]
Access size [ACC_SIZE: 0x07]
High-capacity erase unit size [HC_ERASE_GRP_SIZE: 0x01]
	i.e. 512 KiB
High-capacity erase timeout [ERASE_TIMEOUT_MULT: 0x02]
Reliable write sector count [REL_WR_SEC_C: 0x01]
High-capacity W protect group size [HC_WP_GRP_SIZE: 0x10]
	i.e. 8192 KiB
Sleep current (VCC) [S_C_VCC: 0x08]
Sleep current (VCCQ) [S_C_VCCQ: 0x09]
Sleep/awake timeout [S_A_TIMEOUT: 0x15]
Sector Count [SEC_COUNT: 0x0e8f8000]
	Device is block-addressed
Minimum Write Performance for 8bit:
	[MIN_PERF_W_8_52: 0x8c]
	[MIN_PERF_R_8_52: 0x8c]
	[MIN_PERF_W_8_26_4_52: 0x46]
	[MIN_PERF_R_8_26_4_52: 0x46]
Minimum Write Performance for 4bit:
	[MIN_PERF_W_4_26: 0x1e]
	[MIN_PERF_R_4_26: 0x1e]
Power classes registers:
	[PWR_CL_26_360: 0x00]
	[PWR_CL_52_360: 0x00]
	[PWR_CL_26_195: 0x00]
	[PWR_CL_52_195: 0x00]
Partition switching timing [PARTITION_SWITCH_TIME: 0x01]
Out-of-interrupt busy timing [OUT_OF_INTERRUPT_TIME: 0x0a]
I/O Driver Strength [DRIVER_STRENGTH: 0x1f]
Card Type [CARD_TYPE: 0x57 - 00]
	HS400 Dual Data Rate eMMC @200MHz 1.8VI/O
	HS200 Single Data Rate eMMC @200MHz 1.8VI/O
	HS Dual Data Rate eMMC @52MHz 1.8V or 3VI/O
	HS eMMC @52MHz - at rated device voltage(s)
	HS eMMC @26MHz - at rated device voltage(s)
CSD structure version [CSD_STRUCTURE: 0x02]
Command set [CMD_SET: 0x00]
Command set revision [CMD_SET_REV: 0x00]
Power class [POWER_CLASS: 0x00]
High-speed interface timing [HS_TIMING: 0x03]
Erased memory content [ERASED_MEM_CONT: 0x00]
Boot configuration bytes [PARTITION_CONFIG: 0x00]
	Not boot enable
	No access to boot partition
Boot config protection [BOOT_CONFIG_PROT: 0x00]
Boot bus Conditions [BOOT_BUS_CONDITIONS: 0x00]
High-density erase group definition [ERASE_GROUP_DEF: 0x01]
Boot write protection status registers [BOOT_WP_STATUS]: 0x00
Boot Area Write protection [BOOT_WP]: 0x00
	Power ro locking: possible
	Permanent ro locking: possible
	ro lock status: not locked
User area write protection register [USER_WP]: 0x00
FW configuration [FW_CONFIG]: 0x00
RPMB Size [RPMB_SIZE_MULT]: 0x80
Write reliability setting register [WR_REL_SET]: 0x1f
	user area: the device protects existing data if a power failure occurs during a write operation
	partition 1: the device protects existing data if a power failure occurs during a write operation
	partition 2: the device protects existing data if a power failure occurs during a write operation
	partition 3: the device protects existing data if a power failure occurs during a write operation
	partition 4: the device protects existing data if a power failure occurs during a write operation
Write reliability parameter register [WR_REL_PARAM]: 0x15
	Device supports writing EXT_CSD_WR_REL_SET
	Device supports the enhanced def. of reliable write
Enable background operations handshake [BKOPS_EN]: 0x00
H/W reset function [RST_N_FUNCTION]: 0x00
HPI management [HPI_MGMT]: 0x01
Partitioning Support [PARTITIONING_SUPPORT]: 0x07
	Device support partitioning feature
	Device can have enhanced tech.
Max Enhanced Area Size [MAX_ENH_SIZE_MULT]: 0x00136a
	i.e. 40714240 KiB
Partitions attribute [PARTITIONS_ATTRIBUTE]: 0x00
Partitioning Setting [PARTITION_SETTING_COMPLETED]: 0x00
	Device partition setting NOT complete
General Purpose Partition Size
	[GP_SIZE_MULT_4]: 0x000000
	[GP_SIZE_MULT_3]: 0x000000
	[GP_SIZE_MULT_2]: 0x000000
	[GP_SIZE_MULT_1]: 0x000000
Enhanced User Data Area Size [ENH_SIZE_MULT]: 0x000000
	i.e. 0 KiB
Enhanced User Data Start Address [ENH_START_ADDR]: 0x00000000
	i.e. 0 bytes offset
Bad Block Management mode [SEC_BAD_BLK_MGMNT]: 0x00
Periodic Wake-up [PERIODIC_WAKEUP]: 0x00
Program CID/CSD in DDR mode support [PROGRAM_CID_CSD_DDR_SUPPORT]: 0x01
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[127]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[126]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[125]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[124]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[123]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[122]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[121]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[120]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[119]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[118]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[117]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[116]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[115]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[114]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[113]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[112]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[111]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[110]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[109]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[108]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[107]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[106]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[105]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[104]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[103]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[102]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[101]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[100]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[99]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[98]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[97]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[96]]: 0xff
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[95]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[94]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[93]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[92]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[91]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[90]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[89]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[88]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[87]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[86]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[85]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[84]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[83]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[82]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[81]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[80]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[79]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[78]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[77]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[76]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[75]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[74]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[73]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[72]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[71]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[70]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[69]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[68]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[67]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[66]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[65]]: 0x00
Vendor Specific Fields [VENDOR_SPECIFIC_FIELD[64]]: 0x00
Native sector size [NATIVE_SECTOR_SIZE]: 0x01
	i.e. 4 KiB
Sector size emulation [USE_NATIVE_SECTOR]: 0x00
Sector size [DATA_SECTOR_SIZE]: 0x00
	i.e. 512 B
1st initialization after disabling sector size emulation [INI_TIMEOUT_EMU]: 0x0a
Class 6 commands control [CLASS_6_CTRL]: 0x00
Number of addressed group to be Released[DYNCAP_NEEDED]: 0x00
Exception events control [EXCEPTION_EVENTS_CTRL]: 0x0000
Exception events status[EXCEPTION_EVENTS_STATUS]: 0x0000
Extended Partitions Attribute [EXT_PARTITIONS_ATTRIBUTE]: 0x0000
Context configuration [CONTEXT_CONF[51]]: 0x00
Context configuration [CONTEXT_CONF[50]]: 0x00
Context configuration [CONTEXT_CONF[49]]: 0x00
Context configuration [CONTEXT_CONF[48]]: 0x00
Context configuration [CONTEXT_CONF[47]]: 0x00
Context configuration [CONTEXT_CONF[46]]: 0x00
Context configuration [CONTEXT_CONF[45]]: 0x00
Context configuration [CONTEXT_CONF[44]]: 0x00
Context configuration [CONTEXT_CONF[43]]: 0x00
Context configuration [CONTEXT_CONF[42]]: 0x00
Context configuration [CONTEXT_CONF[41]]: 0x00
Context configuration [CONTEXT_CONF[40]]: 0x00
Context configuration [CONTEXT_CONF[39]]: 0x00
Context configuration [CONTEXT_CONF[38]]: 0x00
Context configuration [CONTEXT_CONF[37]]: 0x00
Packed command status [PACKED_COMMAND_STATUS]: 0x00
Packed command failure index [PACKED_FAILURE_INDEX]: 0x00
Power Off Notification [POWER_OFF_NOTIFICATION]: 0x01
Control to turn the Cache ON/OFF [CACHE_CTRL]: 0x01
Mode config [MODE_CONFIG: 0x00]
Mode operation codes [MODE_OPERATION_CODES: 0x00]
FFU status [FFU_STATUS: 0x00]
	Success
Pre loading data size [PRE_LOADING_DATA_SIZE] is 0 sector size
Max pre loading data size [MAX_PRE_LOADING_DATA_SIZE] is 81428480 sector size
Product state awareness enablement [PRODUCT_STATE_AWARENESS_ENABLEMENT: 0x01]
Secure Removal Type [SECURE_REMOVAL_TYPE: 0x0b]
eMMC Firmware Version: 0003
eMMC Life Time Estimation A [EXT_CSD_DEVICE_LIFE_TIME_EST_TYP_A]: 0x00
eMMC Life Time Estimation B [EXT_CSD_DEVICE_LIFE_TIME_EST_TYP_B]: 0x01
eMMC Pre EOL information [EXT_CSD_PRE_EOL_INFO]: 0x01
Command Queue Support [CMDQ_SUPPORT]: 0x01
Command Queue Depth [CMDQ_DEPTH]: 32
Command Enabled [CMDQ_MODE_EN]: 0x00
`

	info, err := parseGetStorageInfoOutput(context.Background(), []byte(out))
	if err != nil {
		t.Fatal("parseGetStorageInfoOutput() failed: ", err)
	}

	exp := &Info{
		Name:           "hDEaP3",
		Device:         EMMC,
		Status:         Healthy,
		PercentageUsed: 5,
	}

	if !reflect.DeepEqual(info, exp) {
		t.Errorf("parseGetStorageInfoOutput() = %+v; want %+v", info, exp)
	}
}
