// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package policy contains utilities for representing and using policies.
package policy

import (
	"encoding/json"
)

// A Policy is an interface for a more specific policy type.  All the
// concrete policies in this package must implement this interface.
type Policy interface {
	// Inherent properties of the policy.
	Name() string  // Name of policy as it appears in policy_templates.json
	Field() string // groupname.fieldname (or "" for non device policies)
	Scope() Scope  // e.g. User or Device

	// Output of these functions will change depending on the test and value.
	Status() Status                                   // e.g. Set, Unset, Suggested
	UntypedV() interface{}                            // Used to Marshal this policy into JSON
	UnmarshalAs(json.RawMessage) (interface{}, error) // Used to Unmarshal JSON to this type
	Equal(interface{}) bool                           // Used to compare this policy to another
}

// A Scope is a property of the policy and indicates whether it is a
// User or Device policy.
type Scope int

// Definitions for the specific Scope types.
const (
	UserScope   Scope = iota // User policy
	DeviceScope              // Device poicy
)

// A Status indicates how the DMS should serve the policy.  This info
// is used when verifying policies and by the FakeDMS to serve (or not serve
// in the case of Unset or Default) policies.
// It is a combination of Level and Source.
type Status int

// Definitions for the specific Status options.
const (
	SetStatus            Status = iota // Set by DMS as mandatory (default)
	SetRecommendedStatus               // Set by DMS as suggested
	UnsetStatus                        // Not set by DMS (policy value will be ignored)
	DefaultStatus                      // Default value from DMS
)
