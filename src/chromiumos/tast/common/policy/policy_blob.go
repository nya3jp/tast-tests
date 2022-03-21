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
	// Blob by default. This username should usually be used to log into
	// Chrome (i.e. passed in to the Chrome login function).
	DefaultPolicyUser = "tast-user@managedchrome.com"
)

// A Blob is a struct that marshals into what is expected by Chrome's
// fake_dmserver.
type Blob struct {
	UserPolicies          []Policy                                              `json:"-"`
	DeviceProto           enterprise_management_proto.ChromeDeviceSettingsProto `json:"-"`
	DevicePolicies        []Policy                                              `json:"-"`
	PublicAccountPolicies map[string][]Policy                                   `json:"-"`
	ExtensionPM           BlobPolicyMap                                         `json:"-"`
	PolicyUser            string                                                `json:"policy_user"`
	ManagedUsers          []string                                              `json:"managed_users"`
	CurrentKeyIdx         int                                                   `json:"current_key_index,omitempty"`
	RobotAPIAuthCode      string                                                `json:"robot_api_auth_code,omitempty"`
	Licenses              *BlobLicenses                                         `json:"available_licenses,omitempty"`
	TokenEnrollment       *BlobTokenEnrollment                                  `json:"token_enrollment,omitempty"`
	RequestErrors         map[string]int                                        `json:"request_errors,omitempty"`
	AllowDeviceAttrs      bool                                                  `json:"allow_set_device_attributes,omitempty"`
	InitialState          map[string]*BlobInitialState                          `json:"initial_enrollment_state,omitempty"`
	DeviceAffiliationIds  []string                                              `json:"device_affiliation_ids,omitempty"`
	UserAffiliationIds    []string                                              `json:"user_affiliation_ids,omitempty"`
	DirectoryAPIID        string                                                `json:"directory_api_id,omitempty"`
}

// A BlobLicenses struct is a sub-struct used in a Blob.
type BlobLicenses struct {
	Annual    int `json:"annual,omitempty"`
	Perpetual int `json:"perpetual,omitempty"`
}

// A BlobTokenEnrollment struct is a sub-struct used in a Blob.
type BlobTokenEnrollment struct {
	Token    string `json:"token"`
	Username string `json:"username"`
}

// A BlobInitialState struct is a sub-struct used in a Blob.
type BlobInitialState struct {
	EnrollmentMode  int    `json:"initial_enrollment_mode,omitempty"`
	Domain          string `json:"management_domain,omitempty"`
	PackagedLicense bool   `json:"is_license_packaged_with_device,omitempty"`
}

// Entry struct is used to serialize various policies in Blob to JSON format
// accepted by the policy test server.
type Entry struct {
	PolicyType string `json:"policy_type"`
	EntityID   string `json:"entity_id,omitempty"`
	Value      string `json:"value"`
}

// A BlobPolicyMap is a map of policy names to their JSON values.
type BlobPolicyMap map[string]json.RawMessage

// NewBlob returns a simple *Blob. Callers are expected to add user
// and device policies or modify initial setup as desired.
func NewBlob() *Blob {
	return &Blob{
		ManagedUsers:  []string{"*"},
		PolicyUser:    DefaultPolicyUser,
		RequestErrors: make(map[string]int),
	}
}

// AddPolicies adds a given slice of Policy to the Blob.
// Where it goes is based on both the Scope() and Status() of the given policy.
// No action happens if Policy is flagged as Unset or having Default value.
func (pb *Blob) AddPolicies(ps []Policy) error {
	for _, p := range ps {
		if err := pb.AddPolicy(p); err != nil {
			return err
		}
	}
	return nil
}

// AddPolicy adds a given Policy to the Blob.
// Where it goes is based on both the Scope() and Status() of the given policy.
// No action happens if Policy is flagged as Unset or having Default value.
func (pb *Blob) AddPolicy(p Policy) error {
	if p.Status() == StatusUnset || p.Status() == StatusDefault {
		return nil
	}
	switch p.Scope() {
	case ScopeUser:
		pb.UserPolicies = append(pb.UserPolicies, p)
	case ScopeDevice:
		pb.DevicePolicies = append(pb.DevicePolicies, p)
	}
	return nil
}

// AddPublicAccountPolicy adds the given policy to the public account policies associated with the account ID.
// The account ID should match one of the accounts set in the DeviceLocalAccounts policy e.g. tast-user@managedchrome.com.
func (pb *Blob) AddPublicAccountPolicy(accountID string, p Policy) error {
	if p.Scope() != ScopeUser {
		return errors.Errorf("%s is a non-user policy which cannot be added to public accounts", p.Name())
	}

	if pb.PublicAccountPolicies == nil {
		pb.PublicAccountPolicies = make(map[string][]Policy)
	}

	pb.PublicAccountPolicies[accountID] = append(pb.PublicAccountPolicies[accountID], p)

	return nil
}

// AddPublicAccountPolicies adds public policies to the public account policies
// associated with the accountID. The account ID should match one of the
// accounts set in the DeviceLocalAccounts policy.
func (pb *Blob) AddPublicAccountPolicies(accountID string, policies []Policy) error {
	for _, p := range policies {
		if err := pb.AddPublicAccountPolicy(accountID, p); err != nil {
			return errors.Wrapf(err, "could not add policy to the account %s", accountID)
		}
	}
	return nil
}

// AddExtensionPolicy sets the policies for a specific extension.
func (pb *Blob) AddExtensionPolicy(extensionID string, data json.RawMessage) error {
	if pb.ExtensionPM == nil {
		pb.ExtensionPM = make(BlobPolicyMap)
	}

	pb.ExtensionPM[extensionID] = data

	return nil
}

// SetDeviceProto sets the initial device proto with the given proto, this can be used for setting complex proto fields manually.
func (pb *Blob) SetDeviceProto(proto enterprise_management_proto.ChromeDeviceSettingsProto) {
	pb.DeviceProto = proto
}

// MarshalJSON marshals the policy blob into JSON.
// UserPolicies and DevicePolicies can be added using AddPolicies(), then it will be added to "policies" list in the blob.
// PublicAccountPolicies can be added using AddPublicAccountPolicies(), then it will be added to "policies" list.
// DevicePolicies can be set initially to a specific proto manually using SetDeviceProto().
// ExternalPolicies can be added using AddExtensionPolicy(), then it will be added to "external_policies" list in the blob.
func (pb *Blob) MarshalJSON() ([]byte, error) {
	type BlobProxy Blob

	b, err := json.Marshal(BlobProxy(*pb))
	if err != nil {
		return nil, err
	}

	var policies []Entry

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
	policies = append(policies, Entry{
		PolicyType: "google/chromeos/user",
		Value:      base64.StdEncoding.EncodeToString(userOut),
	})

	deviceProto := pb.DeviceProto
	deviceProtoMessage := deviceProto.ProtoReflect()
	for _, p := range pb.DevicePolicies {
		p.SetProto(&deviceProtoMessage)
	}
	deviceOut, err := proto.Marshal(deviceProtoMessage.Interface())
	if err != nil {
		return nil, err
	}
	policies = append(policies, Entry{
		PolicyType: "google/chromeos/device",
		Value:      base64.StdEncoding.EncodeToString(deviceOut),
	})

	if pb.PublicAccountPolicies != nil {
		for k, v := range pb.PublicAccountPolicies {
			paProto := enterprise_management_proto.CloudPolicySettings{}
			paProtoMessage := paProto.ProtoReflect().New()
			for _, p := range v {
				p.SetProto(&paProtoMessage)
			}
			paOut, err := proto.Marshal(paProtoMessage.Interface())
			if err != nil {
				return nil, err
			}
			policies = append(policies, Entry{
				PolicyType: "google/chromeos/publicaccount",
				EntityID:   k,
				Value:      base64.StdEncoding.EncodeToString(paOut),
			})
		}
	}

	m["policies"] = policies

	var externalPolicies []Entry

	if pb.ExtensionPM != nil {
		for id, pJSON := range pb.ExtensionPM {
			exOut, err := pJSON.MarshalJSON()
			if err != nil {
				return nil, err
			}
			externalPolicies = append(externalPolicies, Entry{
				PolicyType: "google/chrome/extension",
				EntityID:   id,
				Value:      base64.StdEncoding.EncodeToString(exOut),
			})
		}
	}

	if len(externalPolicies) > 0 {
		m["external_policies"] = externalPolicies
	}

	return json.Marshal(m)
}
