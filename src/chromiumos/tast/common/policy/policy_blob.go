// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"encoding/base64"
	"encoding/json"

	"google.golang.org/protobuf/proto"

	"chromiumos/policy/chromium/policy/enterprise_management_proto"
	"chromiumos/tast/errors"
)

const (
	// DefaultPolicyUser is the username that will be used for "policy_user" in a
	// PolicyBlob by default. This username should usually be used to log into
	// Chrome (i.e. passed in to the Chrome login function).
	DefaultPolicyUser = "tast-user@managedchrome.com"
)

// A Blob is a struct that marshals into what is expected by Chrome's
// policy_testserver.py.
type Blob struct {
	UserPolicies         []Policy                     `json:"-"`
	DevicePolicies       []Policy                     `json:"-"`
	PAPolicies           map[string][]Policy          `json:"-"`
	UserPs               *BlobUserPolicies            `json:"google/chromeos/user,omitempty"`
	DevicePM             BlobPolicyMap                `json:"google/chromeos/device,omitempty"`
	ExtensionPM          BlobPolicyMap                `json:"google/chromeos/extension,omitempty"`
	PublicAccountPs      map[string]*BlobUserPolicies `json:"-"` // Public account policies are identical to user policies.
	PolicyUser           string                       `json:"policy_user"`
	ManagedUsers         []string                     `json:"managed_users"`
	CurrentKeyIdx        int                          `json:"current_key_index,omitempty"`
	RobotAPIAuthCode     string                       `json:"robot_api_auth_code,omitempty"`
	Licenses             *BlobLicenses                `json:"available_licenses,omitempty"`
	TokenEnrollment      *BlobTokenEnrollment         `json:"token_enrollment,omitempty"`
	RequestErrors        map[string]int               `json:"request_errors,omitempty"`
	AllowDeviceAttrs     bool                         `json:"allow_set_device_attributes,omitempty"`
	InitialState         map[string]*BlobInitialState `json:"initial_enrollment_state,omitempty"`
	DeviceAffiliationIds []string                     `json:"device_affiliation_ids,omitempty"`
	UserAffiliationIds   []string                     `json:"user_affiliation_ids,omitempty"`
	DirectoryAPIID       string                       `json:"directory_api_id,omitempty"`
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

// Field struct is a sub-struct used in a PolicyBlob.
type Field struct {
	PolicyType string `json:"policy_type"`
	EntityID   string `json:"entity_id,omitempty"`
	Value      string `json:"value"`
}

// A BlobPolicyMap is a map of policy names to their JSON values.
type BlobPolicyMap map[string]json.RawMessage

// NewBlob returns a simple *PolicyBlob. Callers are expected to add user
// and device policies or modify initial setup as desired.
func NewBlob() *Blob {
	return &Blob{
		ManagedUsers:  []string{"*"},
		PolicyUser:    DefaultPolicyUser,
		RequestErrors: make(map[string]int),
	}
}

// AddPolicies adds a given slice of Policy to the PolicyBlob.
// Where it goes is based on both the Scope() and Status() of the given
// No action happens if Policy is flagged as Unset or having Default value.
func (pb *Blob) AddPolicies(ps []Policy) error {
	for _, p := range ps {
		if err := pb.AddPolicy(p); err != nil {
			return err
		}
	}
	return nil
}

// AddPolicy adds a given Policy to the PolicyBlob.
// Where it goes is based on both the Scope() and Status() of the given
// No action happens if Policy is flagged as Unset or having Default value.
func (pb *Blob) AddPolicy(p Policy) error {
	if p.Status() == StatusUnset || p.Status() == StatusDefault {
		return nil
	}
	switch p.Scope() {
	case ScopeUser:
		pb.UserPolicies = append(pb.UserPolicies, p)
		if p.Status() == StatusSetRecommended {
			if err := pb.addRecommendedUserPolicy(p); err != nil {
				return err
			}
		} else {
			if err := pb.addMandatoryUserPolicy(p); err != nil {
				return err
			}
		}
	case ScopeDevice:
		pb.DevicePolicies = append(pb.DevicePolicies, p)
		if err := pb.addDevicePolicy(p); err != nil {
			return err
		}
	}
	return nil
}

// AddPublicAccountPolicy adds the given policy to the public account policies associated with the account ID.
// The account ID should match one of the accounts set in the DeviceLocalAccounts policy e.g. tast-user@managedchrome.com.
func (pb *Blob) AddPublicAccountPolicy(accountID string, p Policy) error {
	if p.Scope() != ScopeUser {
		return errors.Errorf("%s is a non-user policy which cannot be added to public accounts", p.Name())
	}

	if pb.PublicAccountPs == nil {
		pb.PublicAccountPs = make(map[string]*BlobUserPolicies)
	}

	if _, ok := pb.PublicAccountPs[accountID]; !ok {
		pb.PublicAccountPs[accountID] = &BlobUserPolicies{}
	}

	policies := pb.PublicAccountPs[accountID]

	if p.Status() == StatusSetRecommended {
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

// AddPublicAccountPolicies adds public policies to the public account policies
// associated with the accountID. The account ID should match one of the
// accounts set in the DeviceLocalAccounts policy.
func (pb *Blob) AddPublicAccountPolicies(accountID string, policies []Policy) error {
	if pb.PAPolicies == nil {
		pb.PAPolicies = make(map[string][]Policy)
	}
	pb.PAPolicies[accountID] = append(pb.PAPolicies[accountID], policies...)
	for _, p := range policies {
		if err := pb.AddPublicAccountPolicy(accountID, p); err != nil {
			return errors.Wrapf(err, "could not add policy to the account %s", accountID)
		}
	}
	return nil
}

// AddLegacyDevicePolicy adds a given one to many legacy device policy to the PolicyBlob.
func (pb *Blob) AddLegacyDevicePolicy(field string, value interface{}) error {
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
func (pb *Blob) MarshalJSON() ([]byte, error) {
	type PolicyBlobProxy Blob

	b, err := json.Marshal(PolicyBlobProxy(*pb))
	if err != nil {
		return nil, err
	}

	var policies []Field

	// if pb.PublicAccountPs == nil {
	// 	return b, nil
	// }

	var m map[string]interface{}
	err = json.Unmarshal(b, &m)
	if err != nil {
		return nil, err
	}

	userProto := enterprise_management_proto.CloudPolicySettings{}
	userProtoMessage := userProto.ProtoReflect().New()
	for _, p := range pb.UserPolicies {
		p.SetProto(&userProtoMessage)
	}

	userOut, err := proto.Marshal(userProtoMessage.Interface())
	if err != nil {
		return nil, err
	}
	// m["google/chromeos/userproto"] = base64.StdEncoding.EncodeToString(userOut)
	policies = append(policies, Field{
		PolicyType: "google/chromeos/user",
		Value:      base64.StdEncoding.EncodeToString(userOut),
	})

	deviceProto := enterprise_management_proto.ChromeDeviceSettingsProto{}
	deviceProtoMessage := deviceProto.ProtoReflect().New()
	for _, p := range pb.DevicePolicies {
		p.SetProto(&deviceProtoMessage)
	}
	deviceOut, err := proto.Marshal(deviceProtoMessage.Interface())
	if err != nil {
		return nil, err
	}
	// m["google/chromeos/deviceproto"] = base64.StdEncoding.EncodeToString(deviceOut)
	policies = append(policies, Field{
		PolicyType: "google/chromeos/device",
		Value:      base64.StdEncoding.EncodeToString(deviceOut),
	})

	if pb.PAPolicies != nil {
		for k, v := range pb.PAPolicies {
			paProto := enterprise_management_proto.CloudPolicySettings{}
			paProtoMessage := paProto.ProtoReflect().New()
			for _, p := range v {
				p.SetProto(&paProtoMessage)
			}
			paOut, err := proto.Marshal(paProtoMessage.Interface())
			if err != nil {
				return nil, err
			}
			// m["google/chromeos/publicaccountproto/"+k] = base64.StdEncoding.EncodeToString(paOut)
			policies = append(policies, Field{
				PolicyType: "google/chromeos/publicaccount",
				EntityID:   k,
				Value:      base64.StdEncoding.EncodeToString(paOut),
			})
		}
	}

	if pb.PublicAccountPs != nil {
		for k, v := range pb.PublicAccountPs {
			m["google/chromeos/publicaccount/"+k] = v
		}
	}

	m["policies"] = policies

	return json.Marshal(m)
}

// addValue tweaks Policy values as needed and then adds them to the given map.
// FakeDMServer expects "policy": "{value}" not "policy": {value} and
// "policy": "[{value}]" not "policy": [{value}], so turn anything that is not
// a bool, int, string, or []string into a string of its JSON representation.
func addValue(p Policy, pm BlobPolicyMap) error {
	v := p.UntypedV()
	vJSON, err := json.Marshal(v)
	if err != nil {
		return errors.Wrapf(err, "could not add %s policy", p.Name())
	}
	switch v.(type) {
	case bool, int, string, []string, []DeviceLocalAccountInfo:
	default:
		vJSON, err = json.Marshal(string(vJSON))
		if err != nil {
			return errors.Wrapf(err, "could not add %s policy", p.Name())
		}
	}
	if p.Scope() == ScopeUser {
		pm[p.Name()] = vJSON
	} else {
		pm[p.Field()] = vJSON
	}
	return nil
}

// addMandatoryUserPolicy adds the given policy as a mandatory user policy.
func (pb *Blob) addMandatoryUserPolicy(p Policy) error {
	if pb.UserPs == nil {
		pb.UserPs = &BlobUserPolicies{}
	}
	if pb.UserPs.MandatoryPM == nil {
		pb.UserPs.MandatoryPM = make(BlobPolicyMap)
	}
	return addValue(p, pb.UserPs.MandatoryPM)
}

// addRecommendedUserPolicy adds the given policy as a recommended user policy.
func (pb *Blob) addRecommendedUserPolicy(p Policy) error {
	if pb.UserPs == nil {
		pb.UserPs = &BlobUserPolicies{}
	}
	if pb.UserPs.RecommendedPM == nil {
		pb.UserPs.RecommendedPM = make(BlobPolicyMap)
	}
	return addValue(p, pb.UserPs.RecommendedPM)
}

// addDevicePolicy adds the given policy as a recommended user policy.
func (pb *Blob) addDevicePolicy(p Policy) error {
	if pb.DevicePM == nil {
		pb.DevicePM = make(BlobPolicyMap)
	}
	return addValue(p, pb.DevicePM)
}
