// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/testing"
)

var (
	deviceVariant = ""
)

type deviceInfo struct {
	ModemVariant string
	Board        string
}

var (
	knownVariants = map[string]deviceInfo{
		"anahera_l850":     {"anahera_l850", "brya"},
		"brya_fm350":       {"brya_fm350", "brya"},
		"brya_l850":        {"brya_l850", "brya"},
		"crota_fm101":      {"crota_fm101", "brya"},
		"primus_l850":      {"primus_l850", "brya"},
		"redrix_fm350":     {"redrix_fm350", "brya"},
		"redrix_l850":      {"redrix_l850", "brya"},
		"vell_fm350":       {"vell_fm350", "brya"},
		"krabby_fm101":     {"krabby_fm101", "corsola"},
		"rusty_fm101":      {"rusty_fm101", "corsola"},
		"steelix_fm101":    {"steelix_fm101", "corsola"},
		"beadrix_nl668am":  {"beadrix_nl668am", "dedede"},
		"boten":            {"boten", "dedede"},
		"bugzzy_l850gl":    {"bugzzy_l850gl", "dedede"},
		"bugzzy_nl668am":   {"bugzzy_nl668am", "dedede"},
		"cret":             {"cret", "dedede"},
		"drawper_l850gl":   {"drawper_l850gl", "dedede"},
		"kracko":           {"kracko", "dedede"},
		"metaknight":       {"metaknight", "dedede"},
		"sasuke":           {"sasuke", "dedede"},
		"sasuke_nl668am":   {"sasuke_nl668am", "dedede"},
		"sasukette":        {"sasukette", "dedede"},
		"storo360_l850gl":  {"storo360_l850gl", "dedede"},
		"storo360_nl668am": {"storo360_nl668am", "dedede"},
		"storo_l850gl":     {"storo_l850gl", "dedede"},
		"storo_nl668am":    {"storo_nl668am", "dedede"},
		"guybrush360_l850": {"guybrush360_l850", "guybrush"},
		"guybrush_fm350":   {"guybrush_fm350", "guybrush"},
		"nipperkin":        {"nipperkin", "guybrush"},
		"evoker":           {"evoker", "herobrine"},
		"herobrine":        {"herobrine", "herobrine"},
		"hoglin":           {"hoglin", "herobrine"},
		"piglin":           {"piglin", "herobrine"},
		"villager":         {"villager", "herobrine"},
		"gooey":            {"gooey", "keeby"},
		"craask_fm101":     {"craask_fm101", "nissa"},
		"nivviks_fm101":    {"nivviks_fm101", "nissa"},
		"pujjo":            {"pujjo", "nissa"},
		"skyrim_fm101":     {"skyrim_fm101", "skyrim"},
		"volteer":          {"volteer", "volteer"},
		"volteer2":         {"volteer2", "volteer"},
		"vilboz":           {"vilboz", "zork"},
		"vilboz360":        {"vilboz360", "zork"},
	}
)

func assignLastIntValueAndDropKey(d LabelMap, to *int, key string) LabelMap {
	if v, ok := getLastIntValue(d, key); ok {
		*to = v
	}
	delete(d, key)
	return d
}

func getLastIntValue(d LabelMap, key string) (int, bool) {
	if s, ok := getLastStringValue(d, key); ok {
		if c, err := strconv.Atoi(s); err == nil {
			return c, true
		}
	}
	return -1, false
}

func assignLastBoolValueAndDropKey(d LabelMap, to *bool, key string) LabelMap {
	if v, ok := getLastBoolValue(d, key); ok {
		*to = v
	}
	delete(d, key)
	return d
}

func getLastBoolValue(d LabelMap, key string) (bool, bool) {
	if s, ok := getLastStringValue(d, key); ok {
		return strings.ToLower(s) == "true", true
	}
	return false, false
}

func assignLastStringValueAndDropKey(d LabelMap, to *string, key string) LabelMap {
	if v, ok := getLastStringValue(d, key); ok {
		*to = v
	}
	delete(d, key)
	return d
}

func getLastStringValue(d LabelMap, key string) (string, bool) {
	if vs, ok := d[key]; ok {
		if len(vs) > 0 {
			return vs[len(vs)-1], true
		}
		return "", false
	}
	return "", false
}

// GetCellularCarrierFromHostInfoLabels return the current carrier name from host_info_labels, else return empty string
func GetCellularCarrierFromHostInfoLabels(ctx context.Context, d LabelMap) string {
	if c, ok := getLastStringValue(d, "carrier"); ok {
		return c
	}
	return ""
}

// GetDevicePoolFromHostInfoLabels return the current device pool name from host_info_labels, else return empty string
func GetDevicePoolFromHostInfoLabels(ctx context.Context, d LabelMap) []string {
	var pools []string
	for _, v := range d["pool"] {
		pools = append(pools, v)
	}
	return pools
}

// EnsureUptime ensures that the system has been up for at least the specified amount of time before returning.
func EnsureUptime(ctx context.Context, duration time.Duration) error {
	uptimeStr, err := ioutil.ReadFile("/proc/uptime")
	if err != nil {
		return errors.Wrap(err, "failed to read system uptime")
	}
	uptimeFloat, err := strconv.ParseFloat(strings.Fields(string(uptimeStr))[0], 64)
	if err != nil {
		return errors.Wrapf(err, "failed to parse system uptime %q", string(uptimeStr))
	}
	uptime := time.Duration(uptimeFloat) * time.Second
	if uptime < duration {
		testing.ContextLogf(ctx, "waiting %s uptime before starting test, current uptime: %s", duration, uptime)
		if err := testing.Sleep(ctx, duration-uptime); err != nil {
			return errors.Wrap(err, "failed to wait for system uptime")
		}
	}
	return nil
}

// GetModemInfoFromHostInfoLabels populate Modem info from host_info_labels
func GetModemInfoFromHostInfoLabels(ctx context.Context, d LabelMap) *ModemInfo {
	var modemInfo ModemInfo

	if c, ok := getLastStringValue(d, "modem_type"); ok {
		modemInfo.Type = c
	}
	if c, ok := getLastStringValue(d, "modem_imei"); ok {
		modemInfo.IMEI = c
	}
	if c, ok := getLastStringValue(d, "modem_supported_bands"); ok {
		modemInfo.SupportedBands = c
	}
	if c, ok := getLastStringValue(d, "modem_sim_count"); ok {
		if v, err := strconv.Atoi(c); err == nil {
			modemInfo.SimCount = v
		} else {
			modemInfo.SimCount = 0
		}
	}
	return &modemInfo
}

// GetSIMInfoFromHostInfoLabels populate SIM info from host_info_labels
func GetSIMInfoFromHostInfoLabels(ctx context.Context, d LabelMap) []*SIMInfo {
	numSim := len(d["sim_slot_id"])
	simInfo := make([]*SIMInfo, numSim)

	for i, v := range d["sim_slot_id"] {
		simID := v
		s := &SIMInfo{}
		if j, err := strconv.Atoi(v); err == nil {
			s.SlotID = j
		}

		lv := "sim_" + simID + "_type"
		d = assignLastStringValueAndDropKey(d, &s.Type, lv)

		lv = "sim_" + simID + "_eid"
		d = assignLastStringValueAndDropKey(d, &s.EID, lv)

		lv = "sim_" + simID + "_test_esim"
		d = assignLastBoolValueAndDropKey(d, &s.TestEsim, lv)

		lv = "sim_" + simID + "_num_profiles"
		numProfiles := 0
		d = assignLastIntValueAndDropKey(d, &numProfiles, lv)

		s.ProfileInfo = make([]*SIMProfileInfo, numProfiles)
		for j := 0; j < numProfiles; j++ {
			s.ProfileInfo[j] = &SIMProfileInfo{}
			profileID := strconv.Itoa(j)
			lv = "sim_" + simID + "_" + profileID + "_iccid"
			d = assignLastStringValueAndDropKey(d, &s.ProfileInfo[j].ICCID, lv)

			lv = "sim_" + simID + "_" + profileID + "_pin"
			d = assignLastStringValueAndDropKey(d, &s.ProfileInfo[j].SimPin, lv)

			lv = "sim_" + simID + "_" + profileID + "_puk"
			d = assignLastStringValueAndDropKey(d, &s.ProfileInfo[j].SimPuk, lv)

			lv = "sim_" + simID + "_" + profileID + "_carrier_name"
			d = assignLastStringValueAndDropKey(d, &s.ProfileInfo[j].CarrierName, lv)
		}
		simInfo[i] = s
	}

	return simInfo
}

// GetLabelsAsStringArray returns the labels as a string array
func GetLabelsAsStringArray(ctx context.Context, cmd func(name string) (val string, ok bool), labelName string) ([]string, error) {
	labelsStr, ok := cmd(labelName)
	if !ok {
		return nil, errors.New("failed to read autotest_host_info_labels")
	}

	var labels []string
	if err := json.Unmarshal([]byte(labelsStr), &labels); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal label string")
	}

	return labels, nil
}

// GetDeviceVariant gets the variant of the device using cros config.
func GetDeviceVariant(ctx context.Context) (string, error) {
	if deviceVariant != "" {
		return deviceVariant, nil
	}
	tempDutVariant, err := crosconfig.Get(ctx, "/modem", "firmware-variant")
	if crosconfig.IsNotFound(err) {
		return "", errors.Wrap(err, "firmware-variant doesn't exist")
	} else if err != nil {
		return "", errors.Wrap(err, "failed to execute cros_config")
	}
	deviceVariant = tempDutVariant
	return deviceVariant, nil
}

// TagKnownBugOnVariant adds a tag to the error code if any of the |variants| matches the DUT's variant.
func TagKnownBugOnVariant(ctx context.Context, errIn error, bugNumber string, variants []string) error {
	dutVariant, err := GetDeviceVariant(ctx)
	if err == nil {
		for _, variant := range variants {
			if dutVariant == variant {
				return errors.Wrapf(err, "known bug on variant: %q bug: %q", variant, bugNumber)
			}
		}
	}
	return errIn
}

// TagKnownBugOnBoard adds a tag to the error code if any of the |boards| matches the DUT's board.
func TagKnownBugOnBoard(ctx context.Context, errIn error, bugNumber string, boards []string) error {
	dutVariant, err := GetDeviceVariant(ctx)
	device, ok := knownVariants[dutVariant]
	if err == nil && ok {
		for _, board := range boards {
			if device.Board == board {
				return errors.Wrapf(errIn, "known bug on board: %q bug: %q", board, bugNumber)
			}
		}
	}
	return errIn
}
