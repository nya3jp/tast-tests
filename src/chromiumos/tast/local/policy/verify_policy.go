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
type DUTPolicies struct {
	Chrome      map[string]*DUTPolicy `json:"chromePolicies"`
	DeviceLocal map[string]*DUTPolicy `json:"deviceLocalAccountPolicies"`
	Extension   map[string]*DUTPolicy `json:"extensionPolicies"`
}

// A DUTPolicy represents the information about a single policy as returned by
// the getAllEnterprisePolicies API.
type DUTPolicy struct {
	Level     string
	Scope     string
	Source    string
	Status    string
	ValueJSON json.RawMessage `json:"value"`
}

func (dp *DUTPolicy) String() string {
	return fmt.Sprintf("{level: %s, scope: %s, source: %s, status: %s, value: %s}",
		dp.Level, dp.Scope, dp.Source, dp.Status, string(dp.ValueJSON))
}

// Constant values as returned by getAllEnterprisePolicies API.
const (
	// kPolicySources
	defaultDUTSource       = "sourceEnterpriseDefault"
	cloudDUTSource         = "cloud"
	adDUTSource            = "sourceActiveDirectory"
	localOverrideDUTSource = "sourceDeviceLocalAccountOverride"
	platformDUTSource      = "platform"
	priorityCloudDUTSource = "priorityCloud"
	mergedDUTSource        = "merged"

	// Scopes
	userDUTScope   = "user"
	deviceDUTScope = "machine"

	// Levels
	mandatoryDUTLevel   = "mandatory"
	recommendedDUTLevel = "recommended"
)

// VerifyPoliciesSet takes a TestAPIConn struct and slice of Policies and
// ensures that Chrome has the given policies set.
// Only the first error is returned.
// Policies with UnsetScope or DefaultScope will be verified as not set or
// set with default source, respectively.
// This function does NOT ensure that other policies are not set on the DUT.
// Only policies passed in are considered, preventing test failures due
// to unrelated default policies.
func VerifyPoliciesSet(ctx context.Context, tconn *chrome.Conn, ps []Policy) error {
	dps, err := policiesFromDUT(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not get policies to compare against")
	}

	// Check only the policies given, ignoring anything else set on the DUT.
	for _, expected := range ps {
		actual, ok := dps.Chrome[expected.Name()]
		// Handle unset policies.
		if expected.Status() == UnsetStatus {
			if ok && actual.Source != defaultDUTSource {
				testing.ContextLogf(ctx, "%s: got %s", expected.Name(), actual)
				return errors.Errorf(
					"%s was set on DUT as a non-default value", expected.Name())
			}
			// Skip any further checking, since there's no actual value to check.
			continue
		}
		if !ok {
			return errors.Errorf("%s was not set on DUT", expected.Name())
		}

		// Compare level and status for set policies.
		switch expected.Status() {
		case SetStatus:
			if actual.Source != cloudDUTSource {
				testing.ContextLogf(ctx, "%s: got %s", expected.Name(), actual)
				return errors.Errorf("%s had a source of %s, not %s",
					expected.Name(), actual.Source, cloudDUTSource)
			}
			if actual.Level != mandatoryDUTLevel {
				testing.ContextLogf(ctx, "%s: got %s", expected.Name(), actual)
				return errors.Errorf("%s had a level of %s, not %s",
					expected.Name(), actual.Level, mandatoryDUTLevel)
			}
		case SetRecommendedStatus:
			if actual.Source != cloudDUTSource {
				testing.ContextLogf(ctx, "%s: got %s", expected.Name(), actual)
				return errors.Errorf("%s had a source of %s, not %s",
					expected.Name(), actual.Source, cloudDUTSource)
			}
			if actual.Level != recommendedDUTLevel {
				testing.ContextLogf(ctx, "%s: got %s", expected.Name(), actual)
				return errors.Errorf("%s had a level of %s, not %s",
					expected.Name(), actual.Level, mandatoryDUTLevel)
			}
		case DefaultStatus:
			if actual.Source != defaultDUTSource {
				testing.ContextLogf(ctx, "%s: got %s", expected.Name(), actual)
				return errors.Errorf("%s had a source of %s, not %s",
					expected.Name(), actual.Source, defaultDUTSource)
			}
			if actual.Level != mandatoryDUTLevel {
				testing.ContextLogf(ctx, "%s: got %s", expected.Name(), actual)
				return errors.Errorf("%s had a level of %s, not %s",
					expected.Name(), actual.Level, mandatoryDUTLevel)
			}
		}

		// Compare scopes.
		if (expected.Scope() == UserScope && actual.Scope != userDUTScope) ||
			(expected.Scope() == DeviceScope && actual.Scope != deviceDUTScope) {
			return errors.Errorf("%s scope mismatch: got %s",
				expected.Name(), actual.Scope)
		}

		// Compare policy value.
		actualValue, err := expected.UnmarshalAs(actual.ValueJSON)
		if err != nil {
			testing.ContextLogf(ctx, "%s: got %s", expected.Name(), actual)
			return errors.Wrapf(err, "%s unmarshal error", expected.Name())
		}
		if !expected.Equal(actualValue) {
			testing.ContextLogf(ctx, "%s: got %s", expected.Name(), actual)
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
