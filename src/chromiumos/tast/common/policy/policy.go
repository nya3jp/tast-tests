// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package policy contains utilities for representing and using policies.
package policy

import (
	"encoding/json"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// Policy is an interface for a more specific policy type.  All the
// concrete policies in this package must implement this interface.
type Policy interface {
	// Name returns the name of the policy as it shows up in chrome://policy.
	// The name is an inherent property of the policy.
	Name() string

	// Scope returns the Scope of this policy, e.g. whether it is a user or
	// device policy. A Scope is an inherent property of the policy.
	Scope() Scope

	// Status returns a Status, e.g. whether the policy is set, unset, or
	// suggested.
	Status() Status

	// UntypedV returns the value of the policy as an interface{} type.
	// It is used to marshal policies into JSON when acting through this
	// interface and the specific type is unknown. Any caller who knows the
	// specific policy type should directly access the value rather than using
	// this function.
	UntypedV() interface{}

	// UnmarshalAs unmarshals a JSON string as this policy's value type,
	// returning either the (interface{} typed) value or an error.
	//
	// As the JSON string is read from the DUT, this function must handle any
	// case where the two value types are mismatched. For example, the
	// ParentAccessControl policy has a object type value that is marked as
	// sensitive. The value saved by this interface will be a struct, but the
	// value in the JSON string read from the DUT will be the string "********".
	UnmarshalAs(json.RawMessage) (interface{}, error)

	// Equal takes an interface{} typed policy value from the DUT (expected to
	// be the output of UnmarshalAs) and returns whether the input matches the
	// value stored in this policy interface.
	//
	// As the input value has been read from the DUT, this function must handle
	// any case where the two values are mismatched. For example, a password
	// string might be masked when it is stored. The value saved by this
	// interface will have the password while the input value will have
	// "********". Those two strings are considered equal in this example.
	Equal(interface{}) bool

	// SetProto sets the proto value of the policy.
	SetProto(*protoreflect.Message)
}

// Scope is a property of the policy and indicates whether it is a User or
// Device policy.
type Scope int

const (
	// ScopeUser (the default type) is used to flag user policies.
	ScopeUser Scope = iota
	// ScopeDevice is used to flag device policies.
	ScopeDevice
)

// String implements the Stringer interface for Scope.
func (s Scope) String() string {
	switch s {
	case ScopeUser:
		return "user"
	case ScopeDevice:
		return "device"
	default:
		return "unknown scope"
	}
}

// Status indicates how the DMS should serve the policy.  This info
// is used when verifying policies and by the FakeDMS to serve (or not serve
// in the case of Unset or Default) policies.
// It is a combination of Level and Source.
type Status int

// Definitions for the specific Status options.
const (
	// StatusSet indicates the policy should be set by DMS as mandatory.
	// It is the default behavior.
	StatusSet Status = iota
	// StatusSetRommended indicates the policy should be set by DMS as suggested.
	StatusSetRecommended
	// StatusUnset indicates the policy should not set by DMS (policy value
	// will be ignored when setting or verifying policies).
	StatusUnset
	// StatusDefault indicates the policy should not be set by DMS but has a
	// default value (value will be ignored when setting policies but not when
	// verifying them).
	StatusDefault
)

// String implements the Stringer interface for Status.
func (s Status) String() string {
	switch s {
	case StatusSet:
		return "set"
	case StatusSetRecommended:
		return "recommended"
	case StatusUnset:
		return "unset"
	case StatusDefault:
		return "default"
	default:
		return "unknown status"
	}
}
