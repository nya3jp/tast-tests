// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package reporters

import (
	"context"
	"regexp"
	"strings"

	"chromiumos/tast/errors"
)

// CrossystemParam represents known Crossystem attributes.
type CrossystemParam string

// Crossystem params used by tests, add more as needed.
const (
	CrossystemParamBackupNvramRequest CrossystemParam = "backup_nvram_request"
	CrossystemParamDevBootUsb         CrossystemParam = "dev_boot_usb"
	CrossystemParamDevDefaultBoot     CrossystemParam = "dev_default_boot"
	CrossystemParamDevBootAltfw       CrossystemParam = "dev_boot_altfw"
	CrossystemParamDevswBoot          CrossystemParam = "devsw_boot"
	CrossystemParamFWBTries           CrossystemParam = "fwb_tries"
	CrossystemParamFWTryNext          CrossystemParam = "fw_try_next"
	CrossystemParamFWTryCount         CrossystemParam = "fw_try_count"
	CrossystemParamFWUpdatetries      CrossystemParam = "fwupdate_tries"
	CrossystemParamFWVboot2           CrossystemParam = "fw_vboot2"
	CrossystemParamKernkeyVfy         CrossystemParam = "kernkey_vfy"
	CrossystemParamLocIdx             CrossystemParam = "loc_idx"
	CrossystemParamMainfwAct          CrossystemParam = "mainfw_act"
	CrossystemParamMainfwType         CrossystemParam = "mainfw_type"
	CrossystemParamWpswCur            CrossystemParam = "wpsw_cur"
	CrossystemParamRecoveryReason     CrossystemParam = "recovery_reason"
)

// RecoveryReason is the recovery reason for the 'crossystem recovery' command.
type RecoveryReason string

// Recovery reasons and their corresponding codes.
const (
	NotRequested         RecoveryReason = "0"
	Legacy               RecoveryReason = "1"
	ROManual             RecoveryReason = "2"
	ROInvalidRW          RecoveryReason = "3"
	ROS3Rresume          RecoveryReason = "4"
	DepROTpmError        RecoveryReason = "5"
	ROSharedData         RecoveryReason = "6"
	ROTestS3             RecoveryReason = "7"
	ROTestLFS            RecoveryReason = "8"
	ROTestLF             RecoveryReason = "9"
	RWNotDone            RecoveryReason = "16"
	RWDevMismatch        RecoveryReason = "17"
	RWRecMismatch        RecoveryReason = "18"
	RWVerifyKeyblock     RecoveryReason = "19"
	RWKeyRollback        RecoveryReason = "20"
	RWDataKeyParse       RecoveryReason = "21"
	RWVerifyPreamble     RecoveryReason = "22"
	RWFwRollback         RecoveryReason = "23"
	RWHeaderValid        RecoveryReason = "24"
	RWGetFwBody          RecoveryReason = "25"
	RWHashWrongSize      RecoveryReason = "26"
	RWVerifyBody         RecoveryReason = "27"
	RWValid              RecoveryReason = "28"
	RWNoRONormal         RecoveryReason = "29"
	ROFirmware           RecoveryReason = "32"
	ROTpmReboot          RecoveryReason = "33"
	ECSoftwareSync       RecoveryReason = "34"
	ECUnknownImage       RecoveryReason = "35"
	DepECHash            RecoveryReason = "36"
	ECExpectedImage      RecoveryReason = "37"
	ECUpdate             RecoveryReason = "38"
	ECJumpRW             RecoveryReason = "39"
	ECProtect            RecoveryReason = "40"
	ROUnspecified        RecoveryReason = "63"
	RWDevScreen          RecoveryReason = "65"
	RWNoOS               RecoveryReason = "66"
	RWInvalidOS          RecoveryReason = "67"
	DepRWTpmError        RecoveryReason = "68"
	RWDevMismatch2       RecoveryReason = "69"
	RWSharedData         RecoveryReason = "70"
	RWTestLK             RecoveryReason = "71"
	DepRWNoDisk          RecoveryReason = "72"
	TPMEFail             RecoveryReason = "73"
	ROTpmSError          RecoveryReason = "80"
	ROTpmWError          RecoveryReason = "81"
	ROTpmLError          RecoveryReason = "82"
	ROTpmUError          RecoveryReason = "83"
	RWTpmRError          RecoveryReason = "84"
	RWTpmWError          RecoveryReason = "85"
	RWTpmLError          RecoveryReason = "86"
	ECHashFailed         RecoveryReason = "87"
	ECHashSize           RecoveryReason = "88"
	LkUnspecified        RecoveryReason = "89"
	RWNoDisk             RecoveryReason = "90"
	RWNoKernel           RecoveryReason = "91"
	RWUnspecified        RecoveryReason = "127"
	KeDmVerity           RecoveryReason = "129"
	KeUnspecified        RecoveryReason = "191"
	UsTestrecoveryReason RecoveryReason = "193"
	UsUnspecified        RecoveryReason = "255"
)

var (
	knownCrossystemParams = []CrossystemParam{
		CrossystemParamDevswBoot,
		CrossystemParamFWBTries,
		CrossystemParamFWTryCount,
		CrossystemParamFWTryNext,
		CrossystemParamFWVboot2,
		CrossystemParamKernkeyVfy,
		CrossystemParamMainfwAct,
		CrossystemParamMainfwType,
		CrossystemParamWpswCur,
	}
	rCrossystemLine = regexp.MustCompile(`^([^ =]*) *= *(.*[^ ]) *# [^#]*$`)
)

// Crossystem returns crossystem output as a map.
// Any required params not found in the output will cause an error.
// You must add `SoftwareDeps: []string{"crossystem"},` to your `testing.Test` to use this.
func (r *Reporter) Crossystem(ctx context.Context, requiredKeys ...CrossystemParam) (map[CrossystemParam]string, error) {
	lines, err := r.CommandOutputLines(ctx, "crossystem")
	if err != nil {
		return nil, err
	}

	parsed, err := parseCrossystemOutput(lines)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse 'crossystem' output")
	}
	for _, k := range requiredKeys {
		if _, found := parsed[k]; !found {
			return nil, errors.Errorf("required param %q not found in output: %v", k, parsed)
		}
	}
	return parsed, nil
}

// CrossystemParam returns the value of the param from crossystem <param> command.
func (r *Reporter) CrossystemParam(ctx context.Context, param CrossystemParam) (string, error) {
	res, err := r.CommandOutput(ctx, "crossystem", string(param))
	if err != nil {
		return "", err
	}
	return string(res), nil
}

// parseCrossystemOutput converts lines of crossystem output to a map.
// Duplicate params will return an error to match behavior in FAFT.
func parseCrossystemOutput(outputLines []string) (map[CrossystemParam]string, error) {
	all := make(map[string]string)
	for _, line := range outputLines {
		kv := rCrossystemLine.FindStringSubmatch(strings.TrimSpace(line))
		if kv == nil {
			return nil, errors.Errorf("failed to parse crossystem line %q", line)
		}
		if _, existing := all[kv[1]]; existing {
			return nil, errors.Errorf("duplicate crossystem param %v, existing value %v, parsing line %q", kv[1], all[kv[1]], line)
		}
		all[kv[1]] = kv[2]
	}

	return filterCrossystemParams(all), nil
}

// filterCrossystemParams removes any param from m that are not known.
func filterCrossystemParams(m map[string]string) map[CrossystemParam]string {
	filtered := make(map[CrossystemParam]string)
	for _, k := range knownCrossystemParams {
		if _, found := m[string(k)]; found {
			filtered[k] = m[string(k)]
		}
	}
	return filtered
}
