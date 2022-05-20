// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"strconv"
	"strings"
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

// GetModemInfoFromHostInfoLabels populate Modem info from host_info_labels
func GetModemInfoFromHostInfoLabels(ctx context.Context, d LabelMap) *ModemInfo {
	var modemInfo ModemInfo

	if c, ok := getLastStringValue(d, "label-modem_type"); ok {
		modemInfo.Type = c
	}
	if c, ok := getLastStringValue(d, "label-modem_imei"); ok {
		modemInfo.Imei = c
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
		d = assignLastStringValueAndDropKey(d, &s.Eid, lv)

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
			d = assignLastStringValueAndDropKey(d, &s.ProfileInfo[j].Iccid, lv)

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
