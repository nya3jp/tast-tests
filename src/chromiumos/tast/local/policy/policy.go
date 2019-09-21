// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package policy contains utilities for representing and using policices.
package policy

import (
	"encoding/json"

	"chromiumos/tast/errors"
)

// A Policy is an interface for a more specific policy type.  All the
// concrete policies in this package must implement this interface.
type Policy interface {
	// Inherent properties of the policy.
	Name() string  // Name of policy as it appears in policy_templates.json
	Field() string // groupname.fieldname (or "" for non device policies)
	Scope() Scope  // e.g. User or Device
	Type() Type    // e.g. Boolean, Int, String, or Dict

	// Output of these functions will change depending on the test and value.
	Status() Status                        // e.g. Set, Unset, Suggested
	UntypedV() interface{}                 // Used to Marshal this policy into JSON
	Compare(json.RawMessage) (bool, error) // Used to compare marshalled JSON
}

// A Scope is a property of the policy and indicates whether it is a
// User or Device policy.
type Scope int

// comment here
const (
	UserScope   Scope = iota // User policy
	DeviceScope              // Device poicy
)

// A Type is a property of the policy and indicates the type of its value.
type Type int

// comment here
const (
	BoolType Type = iota // boolean value
	IntType              // int value
	StrType              // string value
	ListType             // list of strings value
	DictType             // arbitrarily mapped value
)

// A Status indicates how the DMS should serve the policy.  This info
// is used when verifying policies and by the FakeDMS to serve policies.
// It is a combination of Level and Source.
type Status int

// comment here
const (
	SetStatus          Status = iota // Set by DMS as mandatory (default)
	SetSuggestedStatus               // Set by DMS as suggested
	UnsetStatus                      // Not set by DMS (policy value will be ignored)
	DefaultStatus                    // Default value from DMS
)

func boolCompare(m json.RawMessage, v bool) (bool, error) {
	var b bool
	if err := json.Unmarshal(m, &b); err != nil {
		return false, errors.Wrapf(err, "could not read %v as bool", m)
	}
	return b == v, nil
}

func intCompare(m json.RawMessage, v int) (bool, error) {
	var n int
	if err := json.Unmarshal(m, &n); err != nil {
		return false, errors.Wrapf(err, "could not read %v as int", m)
	}
	return n == v, nil
}

func strCompare(m json.RawMessage, v string) (bool, error) {
	var s string
	if err := json.Unmarshal(m, &s); err != nil {
		return false, errors.Wrapf(err, "could not read %v as string", m)
	}
	return s == v, nil
}

func listOrderedCompare(m json.RawMessage, v []string) (bool, error) {
	var l []string
	if err := json.Unmarshal(m, &l); err != nil {
		return false, errors.Wrapf(err, "could not read %v as list of strings", m)
	}
	if len(l) != len(v) {
		return false, nil
	}
	for i := range l {
		if l[i] != v[i] {
			return false, nil
		}
	}
	return true, nil
}
