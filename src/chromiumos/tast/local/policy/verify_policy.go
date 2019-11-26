// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"fmt"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// DUTPolicies represents the format returned from the getAllEnterprisePolicies
// API.
// Each member map matches a string policy name (as shown in chrome://policy,
// not a device policy field name) to a DUTPolicy struct of information on that
// policy.
type DUTPolicies struct {
	Chrome      map[string]*DUTPolicy `json:"chromePolicies"`
	DeviceLocal map[string]*DUTPolicy `json:"deviceLocalAccountPolicies"`
	Extension   map[string]*DUTPolicy `json:"extensionPolicies"`
}

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

// VerifyPolicies takes a TestAPIConn struct and slice of Policies and
// ensures that Chrome has the given policies are set correctly. Only the first
// error is returned.
//
// Policies with StatusUnset or StatusDefault will be verified as not set or
// set with default source, respectively.
// This function does NOT ensure that other policies are not set on the DUT.
// Only policies passed in are considered, preventing test failures due
// to unrelated default policies.
func VerifyPolicies(ctx context.Context, tconn *chrome.Conn, ps []Policy) error {
	dps, err := policiesFromDUT(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not get policies to compare against")
	}

	// Check only the policies given, ignoring anything else set on the DUT.
	for _, expected := range ps {
		actual, ok := dps.Chrome[expected.Name()]
		// Handle policies which are supposed to be unset.
		if expected.Status() == StatusUnset {
			if ok && actual.Source != dutSourceDefault {
				return errors.Errorf(
					"%s was set on DUT from a non-default source with value %s",
					expected.Name(), actual.ValueJSON)
			}
			// Skip any further checking, since there's no actual value to check.
			continue
		}
		if !ok {
			return errors.Errorf("%s was not set on DUT", expected.Name())
		}

		// Flag any set policies with an error value, e.g. schema violations.
		if actual.Error != "" {
			testing.ContextLogf(ctx, "%s: %s", expected.Name(), actual)
			return errors.Errorf("%s error: %s", expected.Name(), actual.Error)
		}

		// Compare status/source.
		switch expected.Status() {
		case StatusSet, StatusSetRecommended:
			if actual.Source != dutSourceCloud {
				return errors.Errorf("%s had a source of %s, not %s",
					expected.Name(), actual.Source, dutSourceCloud)
			}
		case StatusDefault:
			if actual.Source != dutSourceDefault {
				return errors.Errorf("%s had a source of %s, not %s",
					expected.Name(), actual.Source, dutSourceDefault)
			}
		}

		// Compare status/level.
		switch expected.Status() {
		case StatusSet, StatusDefault:
			if actual.Level != dutLevelMandatory {
				return errors.Errorf("%s had a level of %s, not %s",
					expected.Name(), actual.Level, dutLevelMandatory)
			}
		case StatusSetRecommended:
			if actual.Level != dutLevelRecommended {
				return errors.Errorf("%s had a level of %s, not %s",
					expected.Name(), actual.Level, dutLevelRecommended)
			}
		}

		// Compare scope.
		if (expected.Scope() == ScopeUser && actual.Scope != dutScopeUser) ||
			(expected.Scope() == ScopeDevice && actual.Scope != dutScopeDevice) {
			return errors.Errorf("%s scope mismatch: got %s, expected %s",
				expected.Name(), actual.Scope, expected.Scope())
		}

		// Compare policy value.
		actualValue, err := expected.UnmarshalAs(actual.ValueJSON)
		if err != nil {
			return errors.Wrapf(err, "%s unmarshal error", expected.Name())
		}
		if !expected.Equal(actualValue) {
			return errors.Errorf("%s mismatch: got %v, expected %v",
				expected.Name(), actualValue, expected.UntypedV())
		}
	}
	return nil
}

// policiesFromDUT uses the passed in TestAPIConn to call autotestPrivate's
// getAllEnterprisePolicies function.
// For example data, see the Export to JSON button on chrome://policy.
func policiesFromDUT(ctx context.Context, tconn *chrome.Conn) (*DUTPolicies, error) {
	const requestJS = `
		new Promise(function(resolve, reject) {
			chrome.autotestPrivate.getAllEnterprisePolicies(function(policies) {
				if (policies == null) {
					reject("no policies found")
				}
				resolve(policies);
			})
		});`

	var dps DUTPolicies
	if err := tconn.EvalPromise(ctx, requestJS, &dps); err != nil {
		return nil, errors.Wrap(err, "could not get policies from DUT")
	}

	return &dps, nil
}
