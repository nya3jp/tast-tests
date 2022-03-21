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
	// DefaultPolicyUser is the username that is used for "policy_user" in a
	// Blob by default. This username should usually be used to log into
	// Chrome (i.e. passed in to the Chrome login function).
	DefaultPolicyUser = "tast-user@managedchrome.com"

	// PolicyTypeUser is the user policy type.
	PolicyTypeUser = "google/chromeos/user"
	// PolicyTypeDevice is the device policy type.
	PolicyTypeDevice = "google/chromeos/device"
	// PolicyTypePublicAccount is the public account policy type.
	PolicyTypePublicAccount = "google/chromeos/publicaccount"
	// PolicyTypeExtension is the extension policy type.
	PolicyTypeExtension = "google/chrome/extension"
)

// A Blob is a struct that marshals into what is expected by Chrome's
// fake_dmserver.
type Blob struct {
	userPolicies           []Policy                             `json:"-"` // userPolicies can be added using AddPolicies().
	devicePolicies         []Policy                             `json:"-"` // devicePolicies can be added using AddPolicies().
	publicAccountPolicies  map[string][]Policy                  `json:"-"` // publicAccountPolicies can be added using AddPublicAccountPolicies().
	extensionPM            BlobPolicyMap                        `json:"-"` // extensionPM can be added using AddExtensionPolicy().
	DeviceProto            *empb.ChromeDeviceSettingsProto      `json:"-"` // DeviceProto is used to manually set legacy device policies.
	userProto              *empb.CloudPolicySettings            `json:"-"` // userProto is used to manually set user policies for unmarshalling.
	publicAccountsMapProto map[string]*empb.CloudPolicySettings `json:"-"` // publicAccountsProto is used to manually set public accounts policies for unmarshalling.
	AllowDeviceAttrs       bool                                 `json:"allow_set_device_attributes,omitempty"`
	CurrentKeyIdx          int                                  `json:"current_key_index,omitempty"`
	PolicyUser             string                               `json:"policy_user"`
	DirectoryAPIID         string                               `json:"directory_api_id,omitempty"`
	RobotAPIAuthCode       string                               `json:"robot_api_auth_code,omitempty"`
	ManagedUsers           []string                             `json:"managed_users"`
	DeviceAffiliationIds   []string                             `json:"device_affiliation_ids,omitempty"`
	UserAffiliationIds     []string                             `json:"user_affiliation_ids,omitempty"`
	RequestErrors          map[string]int                       `json:"request_errors,omitempty"`
	InitialState           map[string]*BlobInitialState         `json:"initial_enrollment_state,omitempty"`
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
		ManagedUsers:           []string{"*"},
		PolicyUser:             DefaultPolicyUser,
		DeviceProto:            &empb.ChromeDeviceSettingsProto{},
		userProto:              &empb.CloudPolicySettings{},
		publicAccountsMapProto: make(map[string]*empb.CloudPolicySettings),
		publicAccountPolicies:  make(map[string][]Policy),
		extensionPM:            make(BlobPolicyMap),
		RequestErrors:          make(map[string]int),
		InitialState:           make(map[string]*BlobInitialState),
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
		pb.userPolicies = append(pb.userPolicies, p)
	case ScopeDevice:
		pb.devicePolicies = append(pb.devicePolicies, p)
	}
	return nil
}

// AddPublicAccountPolicy adds the given policy to the public account policies
// associated with the account ID. The account ID should match one of the
// accounts set in the DeviceLocalAccounts policy e.g. tast-user@managedchrome.com.
func (pb *Blob) AddPublicAccountPolicy(accountID string, p Policy) error {
	if p.Scope() != ScopeUser {
		return errors.Errorf("%s is a non-user policy which cannot be added to public accounts", p.Name())
	}

	pb.publicAccountPolicies[accountID] = append(pb.publicAccountPolicies[accountID], p)

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
func (pb *Blob) AddExtensionPolicy(extensionID string, data json.RawMessage) {
	pb.extensionPM[extensionID] = data
}

// entry struct is used to serialize various policies in Blob to JSON format
// accepted by the policy test server.
type entry struct {
	PolicyType string `json:"policy_type"`
	EntityID   string `json:"entity_id,omitempty"`
	Value      string `json:"value"`
}

// MarshalJSON marshals the policy blob into JSON.
// userPolicies, devicePolicies and publicAccountPolicies are added to "policies" list in the blob.
// ExternalPolicies are added to "external_policies" list in the blob.
// All the proto values of the policies are encoded to base64.
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

	// Create an empty CloudPolicy proto, then iterate over all
	// the user policies and set their corresponding proto message.
	userProtoMessage := pb.userProto.ProtoReflect()
	for _, p := range pb.userPolicies {
		p.SetProto(&userProtoMessage)
	}
	userOut, err := proto.Marshal(userProtoMessage.Interface())
	if err != nil {
		return nil, err
	}
	policies = append(policies, entry{
		PolicyType: PolicyTypeUser,
		Value:      base64.StdEncoding.EncodeToString(userOut),
	})

	// Retrieve the initial ChromeDevice proto (if it's not set in the test
	// it'll be empty) then iterate over all the device policies and
	// set their corresponding proto messages.
	deviceProtoMessage := pb.DeviceProto.ProtoReflect()
	for _, p := range pb.devicePolicies {
		p.SetProto(&deviceProtoMessage)
	}
	deviceOut, err := proto.Marshal(deviceProtoMessage.Interface())
	if err != nil {
		return nil, err
	}
	policies = append(policies, entry{
		PolicyType: PolicyTypeDevice,
		Value:      base64.StdEncoding.EncodeToString(deviceOut),
	})

	// Add existing public account protos directly.
	for k, p := range pb.publicAccountsMapProto {
		paOut, err := proto.Marshal(p.ProtoReflect().Interface())
		if err != nil {
			return nil, err
		}
		policies = append(policies, entry{
			PolicyType: PolicyTypePublicAccount,
			EntityID:   k,
			Value:      base64.StdEncoding.EncodeToString(paOut),
		})
	}
	// For each public account id, create an empty CloudPolicy proto
	// then iterate over all the user policies associated with
	// the public account id and set their corresponding proto message.
	for k, v := range pb.publicAccountPolicies {
		var publicAccountProto *empb.CloudPolicySettings
		if paProto, ok := pb.publicAccountsMapProto[k]; ok {
			publicAccountProto = paProto
		} else {
			publicAccountProto = &empb.CloudPolicySettings{}
		}
		publicAccountProtoMessage := publicAccountProto.ProtoReflect()
		for _, p := range v {
			p.SetProto(&publicAccountProtoMessage)
		}
		paOut, err := proto.Marshal(publicAccountProtoMessage.Interface())
		if err != nil {
			return nil, err
		}
		policies = append(policies, entry{
			PolicyType: PolicyTypePublicAccount,
			EntityID:   k,
			Value:      base64.StdEncoding.EncodeToString(paOut),
		})
	}

	// Add all the user, device and public account policies to "policies" list
	// in the blob.
	m["policies"] = policies

	var externalPolicies []entry
	// For each extension id, write its associated json.
	for id, pJSON := range pb.extensionPM {
		exOut, err := pJSON.MarshalJSON()
		if err != nil {
			return nil, err
		}
		externalPolicies = append(externalPolicies, entry{
			PolicyType: PolicyTypeExtension,
			EntityID:   id,
			Value:      base64.StdEncoding.EncodeToString(exOut),
		})
	}

	// Add all the extension policies to "external_policies" list in the blob.
	if len(externalPolicies) > 0 {
		m["external_policies"] = externalPolicies
	}

	return json.Marshal(m)
}

// UnmarshalJSON unmarshals JSON into the policy blob.
func (pb *Blob) UnmarshalJSON(jsonData []byte) error {
	type BlobProxy Blob
	var proxyBlob BlobProxy
	err := json.Unmarshal(jsonData, &proxyBlob)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal jsonData to an initial Blob")
	}
	pb.AllowDeviceAttrs = proxyBlob.AllowDeviceAttrs
	pb.CurrentKeyIdx = proxyBlob.CurrentKeyIdx
	pb.PolicyUser = proxyBlob.PolicyUser
	pb.DirectoryAPIID = proxyBlob.DirectoryAPIID
	pb.RobotAPIAuthCode = proxyBlob.RobotAPIAuthCode
	pb.ManagedUsers = proxyBlob.ManagedUsers
	pb.DeviceAffiliationIds = proxyBlob.DeviceAffiliationIds
	pb.UserAffiliationIds = proxyBlob.UserAffiliationIds
	pb.RequestErrors = proxyBlob.RequestErrors
	pb.InitialState = proxyBlob.InitialState

	var objmap map[string]*json.RawMessage
	err = json.Unmarshal(jsonData, &objmap)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal jsonData to a raw map")
	}

	if _, ok := objmap["policies"]; ok {
		var policies []entry
		err = json.Unmarshal(*objmap["policies"], &policies)
		if err != nil {
			return errors.Wrap(err, "failed to unmarshal policies array")
		}
		publicAccountsMapProto := make(map[string]*empb.CloudPolicySettings)
		for _, policy := range policies {
			switch policy.PolicyType {
			case PolicyTypeUser:
				b, err := base64.StdEncoding.DecodeString(policy.Value)
				if err != nil {
					return errors.Wrap(err, "failed to decode the user policy value to base64")
				}
				userProto := &empb.CloudPolicySettings{}
				err = proto.Unmarshal(b, userProto)
				if err != nil {
					return errors.Wrap(err, "failed to unmarshal the proto for the user policy")
				}
				pb.userProto = userProto
			case PolicyTypeDevice:
				b, err := base64.StdEncoding.DecodeString(policy.Value)
				if err != nil {
					return errors.Wrap(err, "failed to decode the device policy value to base64")
				}
				deviceProto := &empb.ChromeDeviceSettingsProto{}
				err = proto.Unmarshal(b, deviceProto)
				if err != nil {
					return errors.Wrap(err, "failed to unmarshal the proto for the device policy")
				}
				pb.DeviceProto = deviceProto
			case PolicyTypePublicAccount:
				b, err := base64.StdEncoding.DecodeString(policy.Value)
				if err != nil {
					return errors.Wrap(err, "failed to decode the public account policy value to base64")
				}
				publicAccountProto := &empb.CloudPolicySettings{}
				err = proto.Unmarshal(b, publicAccountProto)
				if err != nil {
					return errors.Wrap(err, "failed to unmarshal the proto for the public account policy")
				}
				publicAccountsMapProto[policy.EntityID] = publicAccountProto
			default:
				return errors.Wrap(err, "unsupported policy type")
			}
		}
		pb.publicAccountsMapProto = publicAccountsMapProto
	}

	if _, ok := objmap["external_policies"]; ok {
		var externalPolicies []entry
		err = json.Unmarshal(*objmap["external_policies"], &externalPolicies)
		if err != nil {
			return errors.Wrap(err, "failed to unmarshal external_policies array")
		}
		for _, policy := range externalPolicies {
			switch policy.PolicyType {
			case PolicyTypeExtension:
				b, err := base64.StdEncoding.DecodeString(policy.Value)
				if err != nil {
					return errors.Wrap(err, "failed to decode the external policy value to base64")
				}
				var pJSON json.RawMessage
				err = json.Unmarshal(b, &pJSON)
				if err != nil {
					return errors.Wrap(err, "failed to unmarshal the raw json for the external policy")
				}
				pb.extensionPM[policy.EntityID] = pJSON
			default:
				return errors.Wrap(err, "unsupported external policy type")
			}
		}
	}

	return nil
}
