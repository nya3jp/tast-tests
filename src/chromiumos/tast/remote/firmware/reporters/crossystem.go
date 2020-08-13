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

// CrossystemType represents known Crossystem attributes
type CrossystemType string

// Crossystem values used by tests, add more as needed
const (
	CrossystemTypeMainfwType CrossystemType = "mainfw_type"
	CrossystemTypeKernvfyKey CrossystemType = "kernkey_vfy"
)

var knownCrossystemTypes = []CrossystemType{CrossystemTypeMainfwType, CrossystemTypeKernvfyKey}
var rCrossystemLine = regexp.MustCompile(`^([^ =]*) *= *(.*[^ ]) *# [^#]*$`)

// Return crossystem output as a map.  Any required keys not found in the output will cause an error
func (r *reporter) Crossystem(ctx context.Context, requiredKeys ...CrossystemType) (map[CrossystemType]string, error) {
	cRes, cErr := r.CommandLines(ctx, "crossystem")
	if cErr != nil {
		return nil, cErr
	}

	pRes, pErr := parseCrossystemOutput(cRes)
	if pErr != nil {
		return nil, errors.Wrap(pErr, "failed to parse 'crossystem' output")
	}
	for _, k := range requiredKeys {
		if _, found := pRes[k]; !found {
			return nil, errors.Errorf("Required key %q not found in output: %v", k, pRes)
		}
	}
	return pRes, nil
}

// Return the value of the key from crossystem <key> command
func (r *reporter) CrossystemKey(ctx context.Context, key CrossystemType) (string, error) {
	res, err := r.Command(ctx, "crossystem", string(key))
	if err == nil {
		return string(res), nil
	}
	return "", err
}

// parseCrossystemOutput converts lines of crossystem output to a map.  Duplicate keys will return an error to match behavior in faft
func parseCrossystemOutput(outputLines []string) (map[CrossystemType]string, error) {
	temp := make(map[string]string)
	for _, line := range outputLines {
		m := rCrossystemLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			return nil, errors.Errorf("failed to parse line %q", line)
		}
		m1, m2 := string(m[1]), string(m[2])
		if _, existing := temp[m1]; existing {
			return nil, errors.Errorf("duplicate crossystem key %v, existing value %v, parsing line %q", m1, temp[m1], line)
		}
		temp[m1] = m2
	}

	ret := make(map[CrossystemType]string)
	for _, k := range knownCrossystemTypes {
		ret[k] = temp[string(k)]
	}
	return ret, nil
}
