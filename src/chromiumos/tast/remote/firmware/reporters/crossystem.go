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

var rCrossystemLine = regexp.MustCompile(`^([^ =]*) *= *(.*[^ ]) *# [^#]*$`)

// Return crossystem output as a map.  Any required keys not found in the output will cause an error
func (r *reporter) Crossystem(ctx context.Context, requiredKeys ...string) (map[string]string, error) {
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
func (r *reporter) CrossystemKey(ctx context.Context, key string) (string, error) {
	res, err := r.Command(ctx, "crossystem", key)
	if err == nil {
		return string(res), nil
	}
	return "", err
}

// parseCrossystemOutput converts lines of crossystem output to a map.  Duplicate keys will raise an error to match behavior in faft
func parseCrossystemOutput(outputLines []string) (map[string]string, error) {
	ret := make(map[string]string)
	for _, line := range outputLines {
		m := rCrossystemLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			return nil, errors.Errorf("failed to parse line %q", line)
		}
		m1, m2 := string(m[1]), string(m[2])
		if _, existing := ret[m1]; existing {
			return nil, errors.Errorf("duplicate crossystem key %v, existing value %v, parsing line %q", m1, ret[m1], line)
		}
		ret[m1] = m2
	}
	return ret, nil
}
