// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"encoding/base64"
	"encoding/json"

	"google.golang.org/protobuf/proto"

	empb "chromiumos/policy/chromium/policy/enterprise_management_proto"
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
	UserPolicies          []Policy                        `json:"-"` // UserPolicies can be added using AddPolicies().
	DeviceProto           *empb.ChromeDeviceSettingsProto `json:"-"` // DeviceProto is a pointer that can be set manually to device proto, this is mainly used for setting legacy device policies.
	DevicePolicies        []Policy                        `json:"-"` // DevicePolicies can be added using AddPolicies().
	PublicAccountPolicies map[string][]Policy             `json:"-"` // PublicAccountPolicies can be added using AddPublicAccountPolicies().
	ExtensionPM           BlobPolicyMap                   `json:"-"` // ExtensionPM can be added using AddExtensionPolicy().
	AllowDeviceAttrs      bool                            `json:"allow_set_device_attributes,omitempty"`
	CurrentKeyIdx         int                             `json:"current_key_index,omitempty"`
	PolicyUser            string                          `json:"policy_user"`
	DirectoryAPIID        string                          `json:"directory_api_id,omitempty"`
	RobotAPIAuthCode      string                          `json:"robot_api_auth_code,omitempty"`
	ManagedUsers          []string                        `json:"managed_users"`
	DeviceAffiliationIds  []string                        `json:"device_affiliation_ids,omitempty"`
	UserAffiliationIds    []string                        `json:"user_affiliation_ids,omitempty"`
	RequestErrors         map[string]int                  `json:"request_errors,omitempty"`
	InitialState          map[string]*BlobInitialState    `json:"initial_enrollment_state,omitempty"`
}

// A BlobInitialState struct is a sub-struct used in a Blob.
type BlobInitialState struct {
	EnrollmentMode  int    `json:"initial_enrollment_mode,omitempty"`
	Domain          string `json:"management_domain,omitempty"`
	PackagedLicense bool   `json:"is_license_packaged_with_device,omitempty"`
}

// A BlobPolicyMap is a map of policy names to their JSON values.
type BlobPolicyMap map[string]json.RawMessage

// NewBlob returns a simple *Blob. Callers are expected to add user
// and device policies or modify initial setup as desired.
func NewBlob() *Blob {
	return &Blob{
		ManagedUsers: []string{"*"},
		PolicyUser:   DefaultPolicyUser,
		DeviceProto:  &empb.ChromeDeviceSettingsProto{},
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

// entry struct is used to serialize various policies in Blob to JSON format
// accepted by the policy test server.
type entry struct {
	PolicyType string `json:"policy_type"`
	EntityID   string `json:"entity_id,omitempty"`
	Value      string `json:"value"`
}

// MarshalJSON marshals the policy blob into JSON.
// UserPolicies, DevicePolicies and PublicAccountPolicies will be added to "policies" list in the blob.
// ExternalPolicies will be added to "external_policies" list in the blob.
// All the proto values of the policies will be encoded to base64.
func (pb *Blob) MarshalJSON() ([]byte, error) {
	type BlobProxy Blob

	b, err := json.Marshal(BlobProxy(*pb))
	if err != nil {
		return nil, err
	}

	var policies []entry

	var m map[string]interface{}
	err = json.Unmarshal(b, &m)
	if err != nil {
		return nil, err
	}

	// Create an empty CloudPolicy proto, then iterate over all the user policies and set their corresponding proto message.
	userProto := empb.CloudPolicySettings{}
	userProtoMessage := userProto.ProtoReflect().New()
	for _, p := range pb.UserPolicies {
		p.SetProto(&userProtoMessage)
	}
	userOut, err := proto.Marshal(userProtoMessage.Interface())
	if err != nil {
		return nil, err
	}
	policies = append(policies, entry{
		PolicyType: "google/chromeos/user",
		Value:      base64.StdEncoding.EncodeToString(userOut),
	})

	// Retrieve the initial ChromeDevice proto (if it's not set in the test it'll be empty), then iterate over all the device policies and set their corresponding proto message.
	deviceProtoMessage := pb.DeviceProto.ProtoReflect()
	for _, p := range pb.DevicePolicies {
		p.SetProto(&deviceProtoMessage)
	}
	deviceOut, err := proto.Marshal(deviceProtoMessage.Interface())
	if err != nil {
		return nil, err
	}
	policies = append(policies, entry{
		PolicyType: "google/chromeos/device",
		Value:      base64.StdEncoding.EncodeToString(deviceOut),
	})

	// For each public account id, create an empty CloudPolicy proto, then iterate over all the user policies associated with the public account id and set their corresponding proto message.
	if pb.PublicAccountPolicies != nil {
		for k, v := range pb.PublicAccountPolicies {
			publicAccountProto := empb.CloudPolicySettings{}
			publicAccountProtoMessage := publicAccountProto.ProtoReflect().New()
			for _, p := range v {
				p.SetProto(&publicAccountProtoMessage)
			}
			paOut, err := proto.Marshal(publicAccountProtoMessage.Interface())
			if err != nil {
				return nil, err
			}
			policies = append(policies, entry{
				PolicyType: "google/chromeos/publicaccount",
				EntityID:   k,
				Value:      base64.StdEncoding.EncodeToString(paOut),
			})
		}
	}

	// Add all the user, device and public account policies to "policies" list in the blob.
	m["policies"] = policies

	var externalPolicies []entry

	// For each extension id, write its associated json.
	if pb.ExtensionPM != nil {
		for id, pJSON := range pb.ExtensionPM {
			exOut, err := pJSON.MarshalJSON()
			if err != nil {
				return nil, err
			}
			externalPolicies = append(externalPolicies, entry{
				PolicyType: "google/chrome/extension",
				EntityID:   id,
				Value:      base64.StdEncoding.EncodeToString(exOut),
			})
		}
	}

	// Add all the extension policies to "external_policies" list in the blob.
	if len(externalPolicies) > 0 {
		m["external_policies"] = externalPolicies
	}

	return json.Marshal(m)
}
