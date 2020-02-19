// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policyutil

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// A DUTPolicy represents the information about a single policy as returned by
// the getAllEnterprisePolicies API.
// Example JSON: {"scope": "user", "level": "mandatory", "source": "cloud",
//                "value": false, "error": "This policy has been deprecated."}
type DUTPolicy struct {
	Level     string
	Scope     string
	Source    string
	Status    string
	ValueJSON json.RawMessage `json:"value"`
	Error     string
}

// DUTPolicies represents the format returned from the getAllEnterprisePolicies API.
// Each member map matches a string policy name (as shown in chrome://policy,
// not a device policy field name) to a DUTPolicy struct of information on that
// policy.
type DUTPolicies struct {
	Chrome      map[string]*DUTPolicy `json:"chromePolicies"`
	DeviceLocal map[string]*DUTPolicy `json:"deviceLocalAccountPolicies"`
	Extension   map[string]*DUTPolicy `json:"extensionPolicies"`
}

// String turns a DUTPolicy struct into a human readable string.
func (dp *DUTPolicy) String() string {
	return fmt.Sprintf("{level: %s, scope: %s, source: %s, status: %s, value: %s, error: %s}",
		dp.Level, dp.Scope, dp.Source, dp.Status, string(dp.ValueJSON), dp.Error)
}

// Constant values as returned by getAllEnterprisePolicies API.
// These constants are for DUTPolicy members as indicated.
// See policy_conversions.cc in chrome/browser/policy/.
const (
	// Sources (kPolicySources)
	dutSourceDefault       = "sourceEnterpriseDefault"
	dutSourceCloud         = "cloud"
	dutSourceAD            = "sourceActiveDirectory"
	dutSourceLocalOverride = "sourceDeviceLocalAccountOverride"
	dutSourcePlatform      = "platform"
	dutSourcePriorityCloud = "priorityCloud"
	dutSourceMerged        = "merged"

	// Scopes
	dutScopeUser   = "user"
	dutScopeDevice = "machine"

	// Levels
	dutLevelMandatory   = "mandatory"
	dutLevelRecommended = "recommended"
)

// mismatch represents an error found while comparing Policies to DUTPolicies.
type mismatch struct {
	Err error
	Act *DUTPolicy
	Exp policy.Policy
}

// Error implements the error interface.
func (m *mismatch) Error() string {
	return fmt.Sprintf("%s: %v", m.Exp.Name(), m.Err)
}

// Dump returns detailed information about a mismatch.
func (m *mismatch) Dump() string {
	r := fmt.Sprintf("%s: %s\n", m.Exp.Name(), m.Err)
	if m.Act == nil {
		r += fmt.Sprintf("No matching policy found on DUT")
	} else {
		r += fmt.Sprintf("Policy read from DUT: %s\n", m.Act)
	}
	if expVal, err := json.Marshal(m.Exp.UntypedV()); err != nil {
		r += fmt.Sprintf("Could not read expected policy: %v\n", err)
	} else {
		r += fmt.Sprintf("Expected policy: {value: %s, status: %s}\n",
			expVal, m.Exp.Status())
	}
	r += "\n\n" // Add extra newlines as a spacer for easier human reading.
	return r
}

// Verify takes a TestAPIConn struct and slice of Policies and
// ensures that Chrome has the given policies are set correctly. Only the first
// error is returned.
//
// Policies with StatusUnset or StatusDefault will be verified as not set or
// set with default source, respectively.
// This function does NOT ensure that other policies are not set on the DUT.
// Only policies passed in are considered, preventing test failures due
// to unrelated default policies.
func Verify(ctx context.Context, tconn *chrome.Conn, ps []policy.Policy) error {
	var ms []*mismatch
	addM := func(a *DUTPolicy, e policy.Policy, problem string) {
		ms = append(ms, &mismatch{Act: a, Exp: e, Err: errors.New(problem)})
	}

	dps, err := PoliciesFromDUT(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not get policies to compare against")
	}

	// Check only the policies given, ignoring anything else set on the DUT.
	for _, expected := range ps {
		actual, ok := dps.Chrome[expected.Name()]
		if !ok {
			if expected.Status() == policy.StatusUnset {
				// Policy is correctly unset.
				// Skip any further checking since there's nothing to compare.
				continue
			}
			// Policy is unset when it should be set.
			addM(nil, expected, "policy was not set on DUT")
			continue
		}
		if expected.Status() == policy.StatusUnset {
			// Policy is set when it should be unset.
			addM(actual, expected, "policy should not have been set on DUT")
			continue
		}

		// Flag any set policies with an error value, e.g. schema violations.
		if actual.Error != "" {
			addM(actual, expected, "policy error:"+actual.Error)
			continue
		}

		// Compare status/source.
		switch expected.Status() {
		case policy.StatusSet, policy.StatusSetRecommended:
			if actual.Source != dutSourceCloud {
				addM(actual, expected, fmt.Sprintf("saw a source of %s, not %s",
					actual.Source, dutSourceCloud))
			}
		case policy.StatusDefault:
			if actual.Source != dutSourceDefault {
				addM(actual, expected, fmt.Sprintf("saw a source of %s, not %s",
					actual.Source, dutSourceDefault))
			}
		}

		// Compare status/level.
		switch expected.Status() {
		case policy.StatusSet, policy.StatusDefault:
			if actual.Level != dutLevelMandatory {
				addM(actual, expected, fmt.Sprintf("saw a level of %s, not %s",
					actual.Level, dutLevelMandatory))
			}
		case policy.StatusSetRecommended:
			if actual.Level != dutLevelRecommended {
				addM(actual, expected, fmt.Sprintf("saw a level of %s, not %s",
					actual.Level, dutLevelRecommended))
			}
		}

		// Compare scope.
		if (expected.Scope() == policy.ScopeUser && actual.Scope != dutScopeUser) ||
			(expected.Scope() == policy.ScopeDevice && actual.Scope != dutScopeDevice) {
			addM(actual, expected, fmt.Sprintf("saw scope of %s, not %s",
				actual.Scope, expected.Scope()))
		}

		// Compare policy value.
		actualValue, err := expected.UnmarshalAs(actual.ValueJSON)
		if err != nil {
			addM(actual, expected, fmt.Sprintf("value unmarshal error: %v", err))
			continue
		}
		if !expected.Equal(actualValue) {
			addM(actual, expected, "actual value did not match expected")
		}
	}

	if len(ms) == 0 {
		return nil
	}

	// Write detailed information about all errors to file.
	dir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.Wrap(ms[0],
			"found policy errors but couldn't open OutDir for more info - first error")
	}
	const logName = "policy_errors.log"
	logPath := filepath.Join(dir, logName)

	var logs string
	for _, m := range ms {
		logs += m.Dump()
	}
	if err := ioutil.WriteFile(logPath, []byte(logs), 0644); err != nil {
		return errors.Wrapf(ms[0],
			"found policy errors but could not write to logs (%v) - first error", err)
	}

	// Tailor return error based on how many errors were found.
	if len(ms) == 1 {
		return errors.Wrapf(ms[0], "found a policy mismatch (see %s for more info)",
			logName)
	}
	return errors.Wrapf(ms[0], "found %d policy mismatches (see %s for full list) - first error",
		len(ms), logName)
}

// PoliciesFromDUT uses the passed in TestAPIConn to call autotestPrivate's
// getAllEnterprisePolicies function.
// For example data, see the Export to JSON button on chrome://policy.
// Note that a DUTPolicy contains a json.RawMessage value, not an unmarshalled value.
func PoliciesFromDUT(ctx context.Context, tconn *chrome.Conn) (*DUTPolicies, error) {
	const cmd = "tast.promisify(chrome.autotestPrivate.getAllEnterprisePolicies)()"
	var dps DUTPolicies
	if err := tconn.EvalPromise(ctx, cmd, &dps); err != nil {
		return nil, errors.Wrap(err, "could not get policies from DUT")
	}

	return &dps, nil
}
