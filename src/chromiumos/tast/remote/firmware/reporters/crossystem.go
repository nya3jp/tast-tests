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
	CrossystemParamDevswBoot  CrossystemParam = "devsw_boot"
	CrossystemParamFWBTries   CrossystemParam = "fwb_tries"
	CrossystemParamFWTryNext  CrossystemParam = "fw_try_next"
	CrossystemParamFWTryCount CrossystemParam = "fw_try_count"
	CrossystemParamFWVboot2   CrossystemParam = "fw_vboot2"
	CrossystemParamKernkeyVfy CrossystemParam = "kernkey_vfy"
	CrossystemParamMainfwAct  CrossystemParam = "mainfw_act"
	CrossystemParamMainfwType CrossystemParam = "mainfw_type"
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
