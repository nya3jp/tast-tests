// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
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

// boolToXMLBoolean converts a Go boolean to an XML-RPC boolean string.
func boolToXMLBoolean(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

// xmlIntegerToInt converts numeric strings such as '-1' into integers.
func xmlIntegerToInt(xmlInt string) (int, error) {
	if len(xmlInt) == 0 {
		return 0, errors.New("xmlIntegerToInt got empty xml value")
	}
	i, err := strconv.ParseInt(xmlInt, 10, 32)
	if err != nil {
		return 0, err
	}
	return int(i), nil
}

// intToXMLInteger converts a Go integer to an XML-RPC integer string.
func intToXMLInteger(i int) (string, error) {
	if i > math.MaxInt32 || i < math.MinInt32 {
		return "", errors.Errorf("intToXMLInteger needs a value that can fit in an int32: got %d, want between %d and %d", i, math.MinInt32, math.MaxInt32)
	}
	return strconv.FormatInt(int64(i), 10), nil
}

// xmlDoubleToFloat64 converts double-like strings such as "1.5" into float64s.
func xmlDoubleToFloat64(s string) (float64, error) {
	if len(s) == 0 {
		return 0.0, errors.New("xmlDoubleToFloat64 got empty xml value")
	}
	return strconv.ParseFloat(s, 64)
}

// float64ToXMLDouble converts a Go float64 to an XML-RPC double-like string.
func float64ToXMLDouble(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// newValue creates an XML-RPC <value>.
func newValue(in interface{}) (value, error) {
	// TODO(jeffcarp): Support more data types.
	switch v := in.(type) {
	case string:
		return value{String: v}, nil
	case bool:
		return value{Boolean: boolToXMLBoolean(v)}, nil
	case int:
		i, err := intToXMLInteger(v)
		if err != nil {
			return value{}, err
		}
		return value{Int: i}, nil
	case float64:
		return value{Double: float64ToXMLDouble(v)}, nil
	}

	return value{}, errors.Errorf("%q not of supported type", in)
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

// serializeMethodCall turns a method and args into a serialized XML-RPC method call.
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
		return errors.Errorf("response contains %d arg(s); want %d", len(r.Params), len(out))
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
		case *int:
			i, err := xmlIntegerToInt(p.Value.Int)
			if err != nil {
				return err
			}
			*o = i
		case *float64:
			f, err := xmlDoubleToFloat64(p.Value.Double)
			if err != nil {
				return err
			}
			*o = f
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
	servodURL := fmt.Sprintf("http://%s:%d", s.host, s.port)
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

	// If outs are specified, unpack response params.
	// Otherwise, return without unpacking.
	if len(out) > 0 {
		return res.unpack(out)
	}
	return nil
}
