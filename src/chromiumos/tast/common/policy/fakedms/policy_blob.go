// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fakedms

import (
	"encoding/json"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
)

const (
	// DefaultPolicyUser is the username that will be used for "policy_user" in a
	// PolicyBlob by default. This username should usually be used to log into
	// Chrome (i.e. passed in to the Chrome login function).
	DefaultPolicyUser = "tast-user@managedchrome.com"

	defaultInvalidationSource = 16
	defaultInvalidationName   = "test_policy"
)

// A PolicyBlob is a struct that marshals into what is expected by Chrome's
// policy_testserver.py.
type PolicyBlob struct {
	UserPs               *BlobUserPolicies
	DevicePM             BlobPolicyMap
	ExtensionPM          BlobPolicyMap
	PublicAccountPs      map[string]*BlobPublicAccountPolicies
	PolicyUser           string
	ManagedUsers         []string
	CurrentKeyIdx        int
	RobotAPIAuthCode     string
	InvalidationSrc      int
	InvalidationName     string
	Licenses             *BlobLicenses
	TokenEnrollment      *BlobTokenEnrollment
	RequestErrors        map[string]int
	AllowDeviceAttrs     bool
	InitialState         map[string]*BlobInitialState
	DeviceAffiliationIds []string
	UserAffiliationIds   []string
}

// A BlobUserPolicies struct is a sub-struct used in a PolicyBlob.
type BlobUserPolicies struct {
	MandatoryPM   BlobPolicyMap `json:"mandatory,omitempty"`
	RecommendedPM BlobPolicyMap `json:"recommended,omitempty"`
}

// A BlobPublicAccountPolicies struct has the same structure as BlobUserPolicies.
type BlobPublicAccountPolicies BlobUserPolicies

// A BlobLicenses struct is a sub-struct used in a PolicyBlob.
type BlobLicenses struct {
	Annual    int `json:"annual,omitempty"`
	Perpetual int `json:"perpetual,omitempty"`
}

// A BlobTokenEnrollment struct is a sub-struct used in a PolicyBlob.
type BlobTokenEnrollment struct {
	Token    string `json:"token"`
	Username string `json:"username"`
}

// A BlobInitialState struct is a sub-struct used in a PolicyBlob.
type BlobInitialState struct {
	EnrollmentMode  int    `json:"initial_enrollment_mode,omitempty"`
	Domain          string `json:"management_domain,omitempty"`
	PackagedLicense bool   `json:"is_license_packaged_with_device,omitempty"`
}

// A BlobPolicyMap is a map of policy names to their JSON values.
type BlobPolicyMap map[string]json.RawMessage

// NewPolicyBlob returns a simple *PolicyBlob. Callers are expected to add user
// and device policies or modify initial setup as desired.
func NewPolicyBlob() *PolicyBlob {
	return &PolicyBlob{
		ManagedUsers:     []string{"*"},
		PolicyUser:       DefaultPolicyUser,
		InvalidationSrc:  defaultInvalidationSource,
		InvalidationName: defaultInvalidationName,
	}
}

// AddPolicies adds a given slice of Policy to the PolicyBlob.
// Where it goes is based on both the Scope() and Status() of the given policy.
// No action happens if Policy is flagged as Unset or having Default value.
func (pb *PolicyBlob) AddPolicies(ps []policy.Policy) error {
	for _, p := range ps {
		if err := pb.AddPolicy(p); err != nil {
			return err
		}
	}
	return nil
}

// AddPolicy adds a given Policy to the PolicyBlob.
// Where it goes is based on both the Scope() and Status() of the given policy.
// No action happens if Policy is flagged as Unset or having Default value.
func (pb *PolicyBlob) AddPolicy(p policy.Policy) error {
	if p.Status() == policy.StatusUnset || p.Status() == policy.StatusDefault {
		return nil
	}
	switch p.Scope() {
	case policy.ScopeUser:
		if p.Status() == policy.StatusSetRecommended {
			if err := pb.addRecommendedUserPolicy(p); err != nil {
				return err
			}
		} else {
			if err := pb.addMandatoryUserPolicy(p); err != nil {
				return err
			}
		}
	case policy.ScopeDevice:
		if err := pb.addDevicePolicy(p); err != nil {
			return err
		}
	}
	return nil
}

// AddPublicAccountPolicy adds the given policy to the public account policies associated with the account ID.
func (pb *PolicyBlob) AddPublicAccountPolicy(accountID string, p policy.Policy) error {
	if p.Scope() != policy.ScopeUser {
		return errors.Errorf("%s is a non-user policy which cannot be added to public accounts", p.Name())
	}

	if pb.PublicAccountPs == nil {
		pb.PublicAccountPs = make(map[string]*BlobPublicAccountPolicies)
	}

	if _, ok := pb.PublicAccountPs[accountID]; !ok {
		pb.PublicAccountPs[accountID] = &BlobPublicAccountPolicies{}
	}

	policies := pb.PublicAccountPs[accountID]

	if p.Status() == policy.StatusSetRecommended {
		if policies.RecommendedPM == nil {
			policies.RecommendedPM = make(BlobPolicyMap)
		}

		return addValue(p, policies.RecommendedPM)
	}

	if policies.MandatoryPM == nil {
		policies.MandatoryPM = make(BlobPolicyMap)
	}

	return addValue(p, policies.MandatoryPM)
}

// MarshalJSON marshals the policy blob into JSON.
func (pb *PolicyBlob) MarshalJSON() ([]byte, error) {
	j := make(map[string]interface{})

	if pb.UserPs != nil {
		j["google/chromeos/user"] = pb.UserPs
	}

	if pb.DevicePM != nil {
		j["google/chromeos/device"] = pb.DevicePM
	}

	if pb.ExtensionPM != nil {
		j["google/chromeos/extension"] = pb.UserPs
	}

	if pb.PublicAccountPs != nil {
		for k, v := range pb.PublicAccountPs {
			j["google/chromeos/publicaccount/"+k] = v
		}
	}

	j["policy_user"] = pb.PolicyUser
	j["managed_users"] = pb.ManagedUsers

	if pb.CurrentKeyIdx != 0 {
		j["current_key_index"] = pb.CurrentKeyIdx
	}

	if pb.RobotAPIAuthCode != "" {
		j["robot_api_auth_code"] = pb.RobotAPIAuthCode
	}

	j["invalidation_source"] = pb.InvalidationSrc
	j["invalidation_name"] = pb.InvalidationName

	if pb.Licenses != nil {
		j["available_licenses"] = pb.Licenses
	}

	if pb.TokenEnrollment != nil {
		j["token_enrollment"] = pb.TokenEnrollment
	}

	if pb.RequestErrors != nil {
		j["request_errors"] = pb.RequestErrors
	}

	if pb.AllowDeviceAttrs {
		j["allow_set_device_attributes"] = pb.AllowDeviceAttrs
	}

	if pb.InitialState != nil {
		j["initial_enrollment_state"] = pb.InitialState
	}

	if pb.DeviceAffiliationIds != nil {
		j["device_affiliation_ids"] = pb.DeviceAffiliationIds
	}

	if pb.UserAffiliationIds != nil {
		j["user_affiliation_ids"] = pb.UserAffiliationIds
	}

	return json.Marshal(j)
}

// addValue tweaks Policy values as needed and then adds them to the given map.
// FakeDMServer expects "policy": "{value}" not "policy": {value} and
// "policy": "[{value}]" not "policy": [{value}], so turn anything that is not
// a bool, int, string, or []string into a string of its JSON representation.
func addValue(p policy.Policy, pm BlobPolicyMap) error {
	v := p.UntypedV()
	vJSON, err := json.Marshal(v)
	if err != nil {
		return errors.Wrapf(err, "could not add %s policy", p.Name())
	}
	switch v.(type) {
	case bool, int, string, []string:
	default:
		vJSON, err = json.Marshal(string(vJSON))
		if err != nil {
			return errors.Wrapf(err, "could not add %s policy", p.Name())
		}
	}
	if p.Scope() == policy.ScopeUser || p.Scope() == policy.ScopePublicAccount {
		pm[p.Name()] = vJSON
	} else {
		pm[p.Field()] = vJSON
	}
	return nil
}

// addMandatoryUserPolicy adds the given policy as a mandatory user policy.
func (pb *PolicyBlob) addMandatoryUserPolicy(p policy.Policy) error {
	if pb.UserPs == nil {
		pb.UserPs = &BlobUserPolicies{}
	}
	if pb.UserPs.MandatoryPM == nil {
		pb.UserPs.MandatoryPM = make(BlobPolicyMap)
	}
	return addValue(p, pb.UserPs.MandatoryPM)
}

// addRecommendedUserPolicy adds the given policy as a recommended user policy.
func (pb *PolicyBlob) addRecommendedUserPolicy(p policy.Policy) error {
	if pb.UserPs == nil {
		pb.UserPs = &BlobUserPolicies{}
	}
	if pb.UserPs.RecommendedPM == nil {
		pb.UserPs.RecommendedPM = make(BlobPolicyMap)
	}
	return addValue(p, pb.UserPs.RecommendedPM)
}

// addDevicePolicy adds the given policy as a recommended user policy.
func (pb *PolicyBlob) addDevicePolicy(p policy.Policy) error {
	if pb.DevicePM == nil {
		pb.DevicePM = make(BlobPolicyMap)
	}
	return addValue(p, pb.DevicePM)
}
