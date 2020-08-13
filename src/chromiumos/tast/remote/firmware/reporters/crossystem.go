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

// CrossystemKey represents known Crossystem attributes.
type CrossystemKey string

// Crossystem keys used by tests, add more as needed.
const (
	CrossystemKeyMainfwType CrossystemKey = "mainfw_type"
	CrossystemKeyKernkeyVfy CrossystemKey = "kernkey_vfy"
)

var knownCrossystemKeys = []CrossystemKey{CrossystemKeyMainfwType, CrossystemKeyKernkeyVfy}
var rCrossystemLine = regexp.MustCompile(`^([^ =]*) *= *(.*[^ ]) *# [^#]*$`)

// Crossystem returns crossystem output as a map.
// Any required keys not found in the output will cause an error.
func (r *Reporter) Crossystem(ctx context.Context, requiredKeys ...CrossystemKey) (map[CrossystemKey]string, error) {
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
			return nil, errors.Errorf("Required key %q not found in output: %v", k, parsed)
		}
	}
	return parsed, nil
}

// CrossystemKey returns the value of the key from crossystem <key> command.
func (r *Reporter) CrossystemKey(ctx context.Context, key CrossystemKey) (string, error) {
	res, err := r.CommandOutput(ctx, "crossystem", string(key))
	if err != nil {
		return "", err
	}
	return string(res), nil
}

// parseCrossystemOutput converts lines of crossystem output to a map.
// Duplicate keys will return an error to match behavior in FAFT.
func parseCrossystemOutput(outputLines []string) (map[CrossystemKey]string, error) {
	all := make(map[string]string)
	for _, line := range outputLines {
		kv := rCrossystemLine.FindStringSubmatch(strings.TrimSpace(line))
		if kv == nil {
			return nil, errors.Errorf("failed to parse crossystem line %q", line)
		}
		if _, existing := all[kv[1]]; existing {
			return nil, errors.Errorf("duplicate crossystem key %v, existing value %v, parsing line %q", kv[1], all[kv[1]], line)
		}
		all[kv[1]] = kv[2]
	}

	known := make(map[CrossystemKey]string)
	for _, k := range knownCrossystemKeys {
		known[k] = all[string(k)]
	}
	return known, nil
}
