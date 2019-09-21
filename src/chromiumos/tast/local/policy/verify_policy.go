// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// DUTPolicies represents the format returned from the getAllEnterprisePolicices
// API.
type DUTPolicies struct {
	Chrome      map[string]*DUTPolicy `json:"chromePolicies"`
	DeviceLocal map[string]*DUTPolicy `json:"deviceLocalAccountPolicies"`
	Extension   map[string]*DUTPolicy `json:"extensionPolicies"`
}

// A DUTPolicy represents the information about a single policy as returned by
// the getAllEnterprisePolicices API.
type DUTPolicy struct {
	Level  string
	Scope  string
	Source string
	Value  json.RawMessage
}

const (
	defaultDUTSource = "sourceEnterpriseDefault"
)

// VerifyPoliciesSet takes a Chrome struct and list of Policices and ensures
// that Chrome has the given policies set.
// Only compares values, not level, scope, or source. Only the first error is
// returned.
// Note: unless unset policies are passed in, this function does not ensure
// that no other policies are set on the DUT.
func VerifyPoliciesSet(ctx context.Context, cr *chrome.Chrome, ps []Policy) error {
	dps, err := policiesFromDUT(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "could not verify policies")
	}

	for _, expected := range ps {
		actual, ok := dps.Chrome[expected.Name()]
		switch expected.Status() {
		case UnsetStatus:
			if ok && actual.Source != defaultDUTSource {
				return errors.Errorf(
					"%s was set on DUT as a non-default value", expected.Name())
			}
			// Skip any value checking, since there's no value to compare here.
			continue
		case SetStatus, SetSuggestedStatus:
			if !ok {
				return errors.Errorf("%s was not set on DUT", expected.Name())
			}
		case DefaultStatus:
			if !ok {
				return errors.Errorf("%s was not set on DUT", expected.Name())
			}
			if actual.Source != defaultDUTSource {
				return errors.Errorf("%s was not a default value on DUT", expected.Name())
			}
		}

		// TODO mandatory vs. suggested check

		if expected.Scope() == UserScope && actual.Scope != "user" {
			return errors.Errorf("%s scope mismatch: got %s, expected user",
				expected.Name(), actual.Scope)
		}
		if expected.Scope() == DeviceScope && actual.Scope != "device" {
			return errors.Errorf("%s scope mismatch: got %s, expected device",
				expected.Name(), actual.Scope)
		}

		cmp, err := expected.Compare(actual.Value)
		if err != nil {
			return errors.Wrapf(err, "error comparing %s", expected.Name())
		}
		if !cmp {
			return errors.Errorf("%s did not match expected value: got %v, expected %v",
				expected.Name(), actual.Value, expected.UntypedV())
		}
	}
	return nil
}

// policiesFromDUT uses the passed in Chrome struct to call autotestPrivate's
// getAllEnterprisePolicies function.
// For example data, see the Export to JSON button on chrome://policy.
func policiesFromDUT(ctx context.Context, cr *chrome.Chrome) (*DUTPolicies, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not connect to test API")
	}

	const requestJS = `
		new Promise(function(resolve, reject) {
			chrome.autotestPrivate.getAllEnterprisePolicies(function(policies) {
				if (policies == null) {
					reject("no policies found")
				}
				resolve(policies);
			})
		});`

	var dp DUTPolicies
	if err = tconn.EvalPromise(ctx, requestJS, &dp); err != nil {
		return nil, errors.Wrap(err, "could not evaluate request")
	}

	testing.ContextLog(ctx, dp)
	return &dp, nil
}
