// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tape

import "encoding/json"

// requests is a struct representing requests for the setPolicy call to the TAPE server.
type requests struct {
	PolicyTargetKey policyTargetKey `json:"policyTargetKey"`

	PolicyValue policyValue `json:"policyValue"`
	UpdateMask  updateMask  `json:"updateMask"`
}

// policyTargetKey is part of the requests struct and holds the orgunit in which a policy
// will be set.
type policyTargetKey struct {
	TargetResource string `json:"targetResource"`
}

// policyValue is part of the requests struct and holds the uri to the PolicySchema and the values
// which will be set by the setPolicy call.
type policyValue struct {
	PolicySchema string      `json:"policySchema"`
	Value        interface{} `json:"value"`
}

// updateMask is part of the requests struct and holds the names of the parameters which will be set.
type updateMask struct {
	Paths []string `json:"paths"`
}

// PolicySchema is an interface for a more specific policy schema.  All the
// concrete policy schemas in this package must implement this interface.
type PolicySchema interface {
	// Schema2JSON creates the JSON representation of the PolicySchema used in a setPolicy call on the TAPE server.
	Schema2JSON(string) ([]byte, error)
}

// marshalJSON creates a json with a request to set a PolicySchema. It takes an orgunit string representing the orgunit
// in which the PolicySchema should be set. policySchemaURI is the uri of the PolicySchema you want to set and the value
// the PolicySchema will be set to are taken from policySchema which is a struct representation of the PolicySchema that
// will be set. updatePaths is a list of parameters indicating the parameters that will be changed by the request.
func marshalJSON(orgunit, policySchemaURI string, policySchema PolicySchema, updatePaths []string) ([]byte, error) {
	return json.Marshal(&struct {
		Requests requests `json:"requests"`
	}{
		Requests: requests{
			PolicyTargetKey: policyTargetKey{
				TargetResource: "orgunits/" + orgunit,
			},
			PolicyValue: policyValue{
				PolicySchema: policySchemaURI,
				Value:        policySchema,
			},
			UpdateMask: updateMask{
				Paths: updatePaths,
			},
		},
	})
}
