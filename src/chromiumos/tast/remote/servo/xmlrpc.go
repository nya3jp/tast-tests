// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package servo is used to communicate with servo devices connected to DUTs.
// It communicates with servod over XML-RPC.
// More details on servo: https://www.chromium.org/chromium-os/servo
package servo

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"chromiumos/tast/errors"
)

// call represents a Servo call request.
type call struct {
	method string
	args   []interface{}
}

// methodCall mirrors the structure of an XML-RPC method call.
type methodCall struct {
	MethodName string  `xml:"methodName"`
	Params     []param `xml:"params>param"`
}

// request is an XML-RPC request.
type request struct {
	Name   xml.Name `xml:"methodCall"`
	Params []param  `xml:"params>param"`
}

// response is an XML-RPC response.
type response struct {
	Name   xml.Name `xml:"methodResponse"`
	Params []param  `xml:"params>param"`
}

// param is an XML-RPC param.
type param struct {
	Value value `xml:"value"`
}

// value is an XML-RPC value.
type value struct {
	Boolean string `xml:"boolean,omitempty"`
	Double  string `xml:"double,omitempty"`
	Int     string `xml:"int,omitempty"`
	String  string `xml:"string,omitempty"`
}

// xmlBooleanToBool converts the strings '1' or '0' into boolean.
func xmlBooleanToBool(xmlBool string) (bool, error) {
	if len(xmlBool) != 1 {
		return false, errors.Errorf("xmlBooleanToBool got %q; expected '1' or '0'", xmlBool)
	}
	switch xmlBool[0] {
	case '1':
		return true, nil
	case '0':
		return false, nil
	default:
		return false, errors.Errorf("xmlBooleanToBool got %q; expected '1' or '0'", xmlBool)
	}
}

// booleanToXMLBoolean converts a Go boolean to an XML-RPC boolean string.
func boolToXMLBoolean(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

// newValue creates an XML-RPC <value>.
func newValue(in interface{}) (value, error) {
	// TODO(jeffcarp): Support more data types.
	switch v := in.(type) {
	case string:
		return value{String: v}, nil
	case bool:
		return value{Boolean: boolToXMLBoolean(v)}, nil
	}

	return value{}, errors.Errorf("newValue got %q; expected supported type", in)
}

// newParams creates a list of XML-RPC <params>.
func newParams(args []interface{}) ([]param, error) {
	var params []param
	for _, arg := range args {
		v, err := newValue(arg)
		if err != nil {
			return nil, err
		}
		params = append(params, param{v})
	}
	return params, nil
}

func newCall(method string, args ...interface{}) call {
	return call{method, args}
}

// serializeMethodCall turns a method and args into a seralized XML-RPC method call.
func serializeMethodCall(cl call) ([]byte, error) {
	params, err := newParams(cl.args)
	if err != nil {
		return nil, err
	}
	return xml.Marshal(methodCall{cl.method, params})
}

// getTimeout returns the lowest of the default timeout or remaining duration
// to the context's deadline.
func getTimeout(ctx context.Context) time.Duration {
	timeout := rpcTimeout
	if dl, ok := ctx.Deadline(); ok {
		newTimeout := dl.Sub(time.Now())
		// Timeout is min(deadline - now, rpcTimeout).
		if newTimeout < rpcTimeout {
			timeout = newTimeout
		}
	}
	return timeout
}

// unpack extracts a response's arguments into a list of given pointers.
func (r *response) unpack(out []interface{}) error {
	if len(r.Params) != len(out) {
		return errors.Errorf("returned args len (%d) doesn't match out len (%d)", len(r.Params), len(out))
	}

	for i, p := range r.Params {
		switch o := out[i].(type) {
		case *string:
			*o = p.Value.String
		case *bool:
			v, err := xmlBooleanToBool(p.Value.Boolean)
			if err != nil {
				return err
			}
			*o = v
		}
	}

	return nil
}

// run makes an XML-RPC call to servod.
func (s *Servo) run(ctx context.Context, cl call, out ...interface{}) error {
	body, err := serializeMethodCall(cl)
	if err != nil {
		return err
	}

	// Get RPC timeout duration from context or use default.
	timeout := getTimeout(ctx)
	servodURL := fmt.Sprintf("http://%s:%d", s.Host, s.Port)
	httpClient := &http.Client{Timeout: timeout}

	resp, err := httpClient.Post(servodURL, "text/xml", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read body and unmarshal XML.
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	res := response{}
	err = xml.Unmarshal(bodyBytes, &res)
	if err != nil {
		return err
	}

	return res.unpack(out)
}
