// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// ModemfwdJobName is the name of the Modem Firmware Daemon.
const ModemfwdJobName = "modemfwd"

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

func stopJob(ctx context.Context, job string) (bool, error) {
	if !upstart.JobExists(ctx, job) {
		return false, nil
	}
	_, _, pid, err := upstart.JobStatus(ctx, job)
	if err != nil {
		return false, errors.Wrapf(err, "failed to run upstart.JobStatus for %q", job)
	}
	if pid == 0 {
		return false, nil
	}
	err = upstart.StopJob(ctx, job)
	if err != nil {
		return false, errors.Wrapf(err, "failed to stop %q", job)
	}
	return true, nil

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

// StopModemfwd stops the Modem Firmware Daemon if it is currently running and returns true if it was stopped.
func StopModemfwd(ctx context.Context) (bool, error) {
	return stopJob(ctx, ModemfwdJobName)
}

// StartModemfwd starts the Modem Firmware Daemon.
func StartModemfwd(ctx context.Context, debug bool) error {
	debugMode := "false"
	if debug {
		debugMode = "true"
	}

	return upstart.EnsureJobRunning(ctx, ModemfwdJobName, upstart.WithArg("DEBUG_MODE", debugMode))
}

// GetModemInfoFromHostInfoLabels populate Modem info from host_info_labels
func GetModemInfoFromHostInfoLabels(ctx context.Context, d LabelMap) *ModemInfo {
	var modemInfo ModemInfo

	if c, ok := getLastStringValue(d, "label-modem_type"); ok {
		modemInfo.Type = c
	}
	if c, ok := getLastStringValue(d, "label-modem_imei"); ok {
		modemInfo.IMEI = c
	}
	if c, ok := getLastStringValue(d, "label-modem_supported_bands"); ok {
		modemInfo.SupportedBands = c
	}
	if c, ok := getLastStringValue(d, "label-modem_sim_count"); ok {
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
	numSim := len(d["label-sim_slot_id"])
	simInfo := make([]*SIMInfo, numSim)

	for i, v := range d["label-sim_slot_id"] {
		simID := v
		s := &SIMInfo{}
		if j, err := strconv.Atoi(v); err == nil {
			s.SlotID = j
		}

		lv := "label-sim_" + simID + "_type"
		d = assignLastStringValueAndDropKey(d, &s.Type, lv)

		lv = "label-sim_" + simID + "_eid"
		d = assignLastStringValueAndDropKey(d, &s.EID, lv)

		lv = "label-sim_" + simID + "_test_esim"
		d = assignLastBoolValueAndDropKey(d, &s.TestEsim, lv)

		lv = "label-sim_" + simID + "_num_profiles"
		numProfiles := 0
		d = assignLastIntValueAndDropKey(d, &numProfiles, lv)

		s.ProfileInfo = make([]*SIMProfileInfo, numProfiles)
		for j := 0; j < numProfiles; j++ {
			s.ProfileInfo[j] = &SIMProfileInfo{}
			profileID := strconv.Itoa(j)
			lv = "label-sim_" + simID + "_" + profileID + "_iccid"
			d = assignLastStringValueAndDropKey(d, &s.ProfileInfo[j].ICCID, lv)

			lv = "label-sim_" + simID + "_" + profileID + "_pin"
			d = assignLastStringValueAndDropKey(d, &s.ProfileInfo[j].SimPin, lv)

			lv = "label-sim_" + simID + "_" + profileID + "_puk"
			d = assignLastStringValueAndDropKey(d, &s.ProfileInfo[j].SimPuk, lv)

			lv = "label-sim_" + simID + "_" + profileID + "_carrier_name"
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
