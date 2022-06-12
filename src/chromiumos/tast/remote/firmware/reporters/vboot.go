// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file implements a function to report whether the DUT uses Vboot1 or Vboot2.

package reporters

import (
	"context"

	"chromiumos/tast/errors"
)

// RecoveryReasonValue represents recovery_reason attributes.
type RecoveryReasonValue string

// RecoveryReason represents named value for RecoveryReasonValue
type RecoveryReason string

// RecoveryReason params used by tests
const (
	RecoveryReasonNotRequested     RecoveryReason = "NOT_REQUESTED" // Recovery not requested
	RecoveryReasonLegacy           RecoveryReason = "LEGACY" // Recovery requested from legacy utility
	RecoveryReasonROManual         RecoveryReason = "RO_MANUAL" // User manually requested recovery via recovery button
	RecoveryReasonROInvalidRW      RecoveryReason = "RO_INVALID_RW" // RW firmware failed signature check
	RecoveryReasonROS3Resume       RecoveryReason = "RO_S3_RESUME" // S3 resume failed
	RecoveryReasonDepROTPMERROR    RecoveryReason = "DEP_RO_TPM_ERROR" // TPM error in read-only firmware (deprecated)
	RecoveryReasonROSharedData     RecoveryReason = "RO_SHARED_DATA" // Shared data error in read-only firmware
	RecoveryReasonROTestS3         RecoveryReason = "RO_TEST_S3" // Test error from S3Resume()
	RecoveryReasonROTestLFS        RecoveryReason = "RO_TEST_LFS" // Test error from LoadFirmwareSetup()
	RecoveryReasonROTestLF         RecoveryReason = "RO_TEST_LF" // Test error from LoadFirmware()
	RecoveryReasonRWNotDone        RecoveryReason = "RW_NOT_DONE" // RW firmware failed signature check
	RecoveryReasonRWDevMismatch    RecoveryReason = "RW_DEV_MISMATCH"
	RecoveryReasonRWRecMismatch    RecoveryReason = "RW_REC_MISMATCH"
	RecoveryReasonRWVerifyKeyblock RecoveryReason = "RW_VERIFY_KEYBLOCK"
	RecoveryReasonRWKeyRollback    RecoveryReason = "RW_KEY_ROLLBACK"
	RecoveryReasonRWDataKeyParse   RecoveryReason = "RW_DATA_KEY_PARSE"
	RecoveryReasonRWVerifyPreamble RecoveryReason = "RW_VERIFY_PREAMBLE"
	RecoveryReasonRWFWRollback     RecoveryReason = "RW_FW_ROLLBACK"
	RecoveryReasonRWHeaderValid    RecoveryReason = "RW_HEADER_VALID"
	RecoveryReasonRWGetFWBody      RecoveryReason = "RW_GET_FW_BODY"
	RecoveryReasonRWHashWrongSize  RecoveryReason = "RW_HASH_WRONG_SIZE"
	RecoveryReasonRWVerifyBody     RecoveryReason = "RW_VERIFY_BODY"
	RecoveryReasonRWValid          RecoveryReason = "RW_VALID"
	RecoveryReasonRWNoRONormal     RecoveryReason = "RW_NO_RO_NORMAL" // Read-only normal path requested by firmware preamble, but unsupported by firmware.
	RecoveryReasonROFirmware       RecoveryReason = "RO_FIRMWARE" // Firmware boot failure outside of verified boot
	RecoveryReasonROTPMReboot      RecoveryReason = "RO_TPM_REBOOT" // Recovery mode TPM initialization requires a system reboot. The system was already in recovery mode for some other reason when this happened.
	RecoveryReasonECSoftwareSync   RecoveryReason = "EC_SOFTWARE_SYNC" // EC software sync - other error
	RecoveryReasonECUnknownImage   RecoveryReason = "EC_UNKNOWN_IMAGE" // EC software sync - unable to determine active EC image
	RecoveryReasonDepECHash        RecoveryReason = "DEP_EC_HASH" //  EC software sync - error obtaining EC image hash (deprecated)
	RecoveryReasonECExpectedImage  RecoveryReason = "EC_EXPECTED_IMAGE" // EC software sync - error obtaining expected EC image
	RecoveryReasonECUpdate         RecoveryReason = "EC_UPDATE" // EC software sync - error updating EC
	RecoveryReasonECJumpRW         RecoveryReason = "EC_JUMP_RW" // EC software sync - unable to jump to EC-RW
	RecoveryReasonECProtect        RecoveryReason = "EC_PROTECT" // EC software sync - unable to protect / unprotect EC-RW
	RecoveryReasonROUnspecified    RecoveryReason = "RO_UNSPECIFIED" // Unspecified/unknown error in read-only firmware
	RecoveryReasonRWDevScreen      RecoveryReason = "RW_DEV_SCREEN" // User manually requested recovery by pressing a key at developer warning screen.
	RecoveryReasonRWNoOS           RecoveryReason = "RW_NO_OS" // No OS kernel detected
	RecoveryReasonRWInvalidOS      RecoveryReason = "RW_INVALID_OS" // OS kernel failed signature check
	RecoveryReasonDepRWTPMError    RecoveryReason = "DEP_RW_TPM_ERROR" // TPM error in rewritable firmware (deprecated)
	RecoveryReasonRWDevSWOff       RecoveryReason = "RW_DEV_MISMATCH" // RW firmware in dev mode, but dev switch is off.
	RecoveryReasonRWSharedData     RecoveryReason = "RW_SHARED_DATA" // Shared data error in rewritable firmware
	RecoveryReasonRWTestLK         RecoveryReason = "RW_TEST_LK" // Test error from LoadKernel()
	RecoveryReasonDepRWNoDisk      RecoveryReason = "DEP_RW_NO_DISK" // No bootable disk found (deprecated)
	RecoveryReasonTPMEFail         RecoveryReason = "TPM_E_FAIL" // Rebooting did not correct TPM_E_FAIL or TPM_E_FAILEDSELFTEST
	RecoveryReasonROTPMSError      RecoveryReason = "RO_TPM_S_ERROR" // TPM setup error in read-only firmware
	RecoveryReasonROTPMWError      RecoveryReason = "RO_TPM_W_ERROR" // TPM write error in read-only firmware
	RecoveryReasonROTPMLError      RecoveryReason = "RO_TPM_L_ERROR" // TPM lock error in read-only firmware
	RecoveryReasonROTPMUError      RecoveryReason = "RO_TPM_U_ERROR" // TPM update error in read-only firmware
	RecoveryReasonRWTPMRError      RecoveryReason = "RW_TPM_R_ERROR" // TPM read error in rewritable firmware
	RecoveryReasonRWTPMWError      RecoveryReason = "RW_TPM_W_ERROR" // TPM write error in rewritable firmware
	RecoveryReasonRWTPMLError      RecoveryReason = "RW_TPM_L_ERROR" // TPM lock error in rewritable firmware
	RecoveryReasonECHashFailed     RecoveryReason = "EC_HASH_FAILED" // EC software sync unable to get EC image hash
	RecoveryReasonECHashSize       RecoveryReason = "EC_HASH_SIZE" // EC software sync invalid image hash size
	RecoveryReasonLKUnspecified    RecoveryReason = "LK_UNSPECIFIED" // Unspecified error while trying to load kernel
	RecoveryReasonRWNoDisk         RecoveryReason = "RW_NO_DISK" // No bootable storage device in system
	RecoveryReasonRWNoKernel       RecoveryReason = "RW_NO_KERNEL" // No bootable kernel found on disk
	RecoveryReasonRWUnspecified    RecoveryReason = "RW_UNSPECIFIED" // Unspecified/unknown error in rewritable firmware
	RecoveryReasonKEDMVerity       RecoveryReason = "KE_DM_VERITY" // DM-verity error
	RecoveryReasonKEUnspecified    RecoveryReason = "KE_UNSPECIFIED" // Unspecified/unknown error in kernel
	RecoveryReasonUSTest           RecoveryReason = "US_TEST" // Recovery mode test from user-mode
	RecoveryReasonUSUnspecified    RecoveryReason = "US_UNSPECIFIED" // Unspecified/unknown error in user-mode
)

// Mapping recovery reason codes to named value used by test
var recoveryReasonCodesMap = map[RecoveryReasonValue]RecoveryReason{
	"0":   RecoveryReasonNotRequested,
	"1":   RecoveryReasonLegacy,
	"2":   RecoveryReasonROManual,
	"3":   RecoveryReasonROInvalidRW,
	"4":   RecoveryReasonROS3Resume,
	"5":   RecoveryReasonDepROTPMERROR,
	"6":   RecoveryReasonROSharedData,
	"7":   RecoveryReasonROTestS3,
	"8":   RecoveryReasonROTestLFS,
	"9":   RecoveryReasonROTestLF,
	"16":  RecoveryReasonRWNotDone,
	"17":  RecoveryReasonRWDevMismatch,
	"18":  RecoveryReasonRWRecMismatch,
	"19":  RecoveryReasonRWVerifyKeyblock,
	"20":  RecoveryReasonRWKeyRollback,
	"21":  RecoveryReasonRWDataKeyParse,
	"22":  RecoveryReasonRWVerifyPreamble,
	"23":  RecoveryReasonRWFWRollback,
	"24":  RecoveryReasonRWHeaderValid,
	"25":  RecoveryReasonRWGetFWBody,
	"26":  RecoveryReasonRWHashWrongSize,
	"27":  RecoveryReasonRWVerifyBody,
	"28":  RecoveryReasonRWValid,
	"29":  RecoveryReasonRWNoRONormal,
	"32":  RecoveryReasonROFirmware,
	"33":  RecoveryReasonROTPMReboot,
	"34":  RecoveryReasonECSoftwareSync,
	"35":  RecoveryReasonECUnknownImage,
	"36":  RecoveryReasonDepECHash,
	"37":  RecoveryReasonECExpectedImage,
	"38":  RecoveryReasonECUpdate,
	"39":  RecoveryReasonECJumpRW,
	"40":  RecoveryReasonECProtect,
	"63":  RecoveryReasonROUnspecified,
	"65":  RecoveryReasonRWDevScreen,
	"66":  RecoveryReasonRWNoOS,
	"67":  RecoveryReasonRWInvalidOS,
	"68":  RecoveryReasonDepRWTPMError,
	"69":  RecoveryReasonRWDevSWOff,
	"70":  RecoveryReasonRWSharedData,
	"71":  RecoveryReasonRWTestLK,
	"72":  RecoveryReasonDepRWNoDisk,
	"73":  RecoveryReasonTPMEFail,
	"80":  RecoveryReasonROTPMSError,
	"81":  RecoveryReasonROTPMWError,
	"82":  RecoveryReasonROTPMLError,
	"83":  RecoveryReasonROTPMUError,
	"84":  RecoveryReasonRWTPMRError,
	"85":  RecoveryReasonRWTPMWError,
	"86":  RecoveryReasonRWTPMLError,
	"87":  RecoveryReasonECHashFailed,
	"88":  RecoveryReasonECHashSize,
	"89":  RecoveryReasonLKUnspecified,
	"90":  RecoveryReasonRWNoDisk,
	"91":  RecoveryReasonRWNoKernel,
	"127": RecoveryReasonRWUnspecified,
	"129": RecoveryReasonKEDMVerity,
	"191": RecoveryReasonKEUnspecified,
	"193": RecoveryReasonUSTest,
	"255": RecoveryReasonUSUnspecified,
}

// Vboot2 determines whether the DUT's current firmware was selected by vboot2.
func (r *Reporter) Vboot2(ctx context.Context) (bool, error) {
	csValue, err := r.CrossystemParam(ctx, CrossystemParamFWVboot2)
	if err != nil {
		return false, err
	}
	return parseFWVboot2(csValue)
}

// parseFWVboot2 determines whether a fw_vboot2 crossystem value represents a vboot2 DUT.
func parseFWVboot2(value string) (bool, error) {
	if value != "1" && value != "0" {
		return false, errors.Errorf("unexpected fw_vboot2 value: want 1 or 0; got %q", value)
	}
	return value == "1", nil
}

func (r *Reporter) ContainsRecoveryReason(ctx context.Context, expectedReasons []RecoveryReason) (bool, error) {
	if csRecReason, err := r.CrossystemParam(ctx, CrossystemParamRecoveryReason); err != nil {
		return false, errors.Wrapf(err, "failed to get recovery reason")
	} else {
		for _, expReason := range expectedReasons {
			if recoveryReasonCodesMap[RecoveryReasonValue(csRecReason)] == expReason {
				return true, nil
			}
		}
	}
	return false, nil
}
