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
	DefaultPolicyUser         = "tast-user@managedchrome.com"
	defaultInvalidationSource = 16
	defaultInvalidationName   = "test_policy"
)

// A PolicyBlob is a struct that marshals into what is expected by Chrome's
// policy_testserver.py.
type PolicyBlob struct {
	UserPs               *BlobUserPolicies            `json:"google/chromeos/user,omitempty"`
	DevicePM             BlobPolicyMap                `json:"google/chromeos/device,omitempty"`
	ExtensionPM          BlobPolicyMap                `json:"google/chromeos/extension,omitempty"`
	PublicAccountPs      map[string]*BlobUserPolicies `json:"-"` // Public account policies are identical to user policies.
	PolicyUser           string                       `json:"policy_user"`
	ManagedUsers         []string                     `json:"managed_users"`
	CurrentKeyIdx        int                          `json:"current_key_index,omitempty"`
	RobotAPIAuthCode     string                       `json:"robot_api_auth_code,omitempty"`
	InvalidationSrc      int                          `json:"invalidation_source"`
	InvalidationName     string                       `json:"invalidation_name"`
	Licenses             *BlobLicenses                `json:"available_licenses,omitempty"`
	TokenEnrollment      *BlobTokenEnrollment         `json:"token_enrollment,omitempty"`
	RequestErrors        map[string]int               `json:"request_errors,omitempty"`
	AllowDeviceAttrs     bool                         `json:"allow_set_device_attributes,omitempty"`
	InitialState         map[string]*BlobInitialState `json:"initial_enrollment_state,omitempty"`
	DeviceAffiliationIds []string                     `json:"device_affiliation_ids,omitempty"`
	UserAffiliationIds   []string                     `json:"user_affiliation_ids,omitempty"`
}

// A BlobUserPolicies struct is a sub-struct used in a PolicyBlob.
type BlobUserPolicies struct {
	MandatoryPM   BlobPolicyMap `json:"mandatory,omitempty"`
	RecommendedPM BlobPolicyMap `json:"recommended,omitempty"`
}

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
// The account ID should match one of the accounts set in the DeviceLocalAccounts policy e.g. tast-user@managedchrome.com.
func (pb *PolicyBlob) AddPublicAccountPolicy(accountID string, p policy.Policy) error {
	if p.Scope() != policy.ScopeUser {
		return errors.Errorf("%s is a non-user policy which cannot be added to public accounts", p.Name())
	}

	if pb.PublicAccountPs == nil {
		pb.PublicAccountPs = make(map[string]*BlobUserPolicies)
	}

	if _, ok := pb.PublicAccountPs[accountID]; !ok {
		pb.PublicAccountPs[accountID] = &BlobUserPolicies{}
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

// AddLegacyDevicePolicy adds a given one to many legacy device policy to the PolicyBlob.
func (pb *PolicyBlob) AddLegacyDevicePolicy(field string, value interface{}) error {
	if pb.DevicePM == nil {
		pb.DevicePM = make(BlobPolicyMap)
	}

	vJSON, err := json.Marshal(value)
	if err != nil {
		return errors.Wrapf(err, "could not marshal the %s field", field)
	}
	pb.DevicePM[field] = vJSON

	return nil
}

// MarshalJSON marshals the policy blob into JSON. PublicAccountPs needs special
// handling as the key is based on the account ID. To work around this, we first
// marshal and unmarshal pb into a map which omits PublicAccountPs, and add the
// public account policies to the map afterwards.
func (pb *PolicyBlob) MarshalJSON() ([]byte, error) {
	type PolicyBlobProxy PolicyBlob

	b, err := json.Marshal(PolicyBlobProxy(*pb))
	if err != nil {
		return nil, err
	}

	if pb.PublicAccountPs == nil {
		return b, nil
	}

	var m map[string]interface{}
	err = json.Unmarshal(b, &m)
	if err != nil {
		return nil, err
	}

	for k, v := range pb.PublicAccountPs {
		m["google/chromeos/publicaccount/"+k] = v
	}

	return json.Marshal(m)
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
	case bool, int, string, []string, []policy.DeviceLocalAccountInfo:
	default:
		vJSON, err = json.Marshal(string(vJSON))
		if err != nil {
			return errors.Wrapf(err, "could not add %s policy", p.Name())
		}
	}
	if p.Scope() == policy.ScopeUser {
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
