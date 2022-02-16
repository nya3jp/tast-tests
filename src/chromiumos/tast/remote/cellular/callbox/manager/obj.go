// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package manager

import "encoding/json"

// RequestBody is a request object that can be marshalled into a byte array.
type RequestBody interface {
	// Marshall marshals this object into a byte array.
	Marshall() ([]byte, error)
}

// ConfigureCallboxRequestBody is the request body for ConfigureCallbox requests.
type ConfigureCallboxRequestBody struct {
	Callbox       string
	Hardware      string
	CellularType  string
	ParameterList []string
}

// Marshall marshals this object into a byte array.
func (r *ConfigureCallboxRequestBody) Marshall() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"callbox":        r.Callbox,
		"hardware":       r.Hardware,
		"cellular_type":  r.CellularType,
		"parameter_list": r.ParameterList,
	})
}

// BeginSimulationRequestBody is the request body for BeginSimulation requests.
type BeginSimulationRequestBody struct {
	Callbox string
}

// Marshall marshals this object into a byte array.
func (r *BeginSimulationRequestBody) Marshall() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"callbox": r.Callbox,
	})
}

// SendSmsRequestBody is the request body for SendSms requests.
type SendSmsRequestBody struct {
	Callbox string
	Message string
}

// Marshall marshals this object into a byte array.
func (r *SendSmsRequestBody) Marshall() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"callbox": r.Callbox,
		"sms":     r.Message,
	})
}
