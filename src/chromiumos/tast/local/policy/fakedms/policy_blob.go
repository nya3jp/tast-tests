// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fakedms

import (
	"chromiumos/tast/local/policy"
)

// A PolicyBlob is a struct that marshals into what is expected by Chrome's
// policy_testserver.py.
type PolicyBlob struct {
	UserPs           *BlobUserPolicies    `json:"google/chromeos/user,omitempty"`
	DevicePM         BlobPolicyMap        `json:"google/chromeos/device,omitempty"`
	ExtensionPM      BlobPolicyMap        `json:"google/chromeos/extension,omitempty"`
	PolicyUser       string               `json:"policy_user"`
	ManagedUsers     []string             `json:"managed_users"`
	CurrentKeyIdx    int                  `json:"current_key_index,omitempty"`
	RobotAPIAuthCode string               `json:"robot_api_auth_code,omitempty"`
	InvalidationSrc  int                  `json:"invalidation_source"`
	InvalidationName string               `json:"invalidation_name"`
	Licenses         *BlobLicensesInfo    `json:"available_licenses,omitempty"`
	TokenEnrollment  *BlobTokenEnrollment `json:"token_enrollment,omitempty"`
}

// A BlobUserPolicies struct is a sub-struct used in a PolicyBlob.
type BlobUserPolicies struct {
	MandatoryPM   BlobPolicyMap `json:"mandatory,omitempty"`
	RecommendedPM BlobPolicyMap `json:"recommended,omitempty"`
}

// A BlobLicensesInfo struct is a sub-struct used in a PolicyBlob.
type BlobLicensesInfo struct {
	Annual    int `json:"annual,omitempty"`
	Perpetual int `json:"perpetual,omitempty"`
}

// A BlobTokenEnrollment struct is a sub-struct used in a PolicyBlob.
type BlobTokenEnrollment struct {
	Token    string `json:"token"`
	Username string `json:"username"`
}

// A BlobPolicyMap is a map of type string policy names to their values.
type BlobPolicyMap map[string]interface{}

// NewPolicyBlob returns default *PolicyBlob. Callers are expected to add user and device
// policies seperately as desired.
func NewPolicyBlob() *PolicyBlob {
	return &PolicyBlob{
		ManagedUsers:     []string{"*"},
		PolicyUser:       "tast-user@managedchrome.com",
		InvalidationSrc:  16,
		InvalidationName: "test_policy",
	}
}

// AddPolicy adds a given policy to the PolicyBlob.
// Where it goes is based on both the Scope() and Status() of the given policy.
func (pb *PolicyBlob) AddPolicy(p policy.Policy) {
	if p.Status() == policy.UnsetStatus || p.Status() == policy.DefaultStatus {
		return
	}
	switch p.Scope() {
	case policy.UserScope:
		if p.Status() == policy.SetSuggestedStatus {
			pb.addSuggestedUserPolicy(p)
		} else {
			pb.addMandatoryUserPolicy(p)
		}
	case policy.DeviceScope:
		pb.addDevicePolicy(p)
	}
}

func (pb *PolicyBlob) addMandatoryUserPolicy(p policy.Policy) {
	if pb.UserPs == nil {
		pb.UserPs = &BlobUserPolicies{}
	}
	if pb.UserPs.MandatoryPM == nil {
		pb.UserPs.MandatoryPM = make(BlobPolicyMap)
	}
	pb.UserPs.MandatoryPM[p.Name()] = p.UntypedV()
}

func (pb *PolicyBlob) addSuggestedUserPolicy(p policy.Policy) {
	if pb.UserPs == nil {
		pb.UserPs = &BlobUserPolicies{}
	}
	if pb.UserPs.RecommendedPM == nil {
		pb.UserPs.RecommendedPM = make(BlobPolicyMap)
	}
	pb.UserPs.RecommendedPM[p.Name()] = p.UntypedV()
}

func (pb *PolicyBlob) addDevicePolicy(p policy.Policy) {
	if pb.DevicePM == nil {
		pb.DevicePM = make(BlobPolicyMap)
	}
	pb.DevicePM[p.Field()] = p.UntypedV()
}
