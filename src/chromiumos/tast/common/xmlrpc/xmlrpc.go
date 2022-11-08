// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package xmlrpc implements the XML-RPC client library.
package xmlrpc

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const defaultRPCTimeout = 10 * time.Second

// XMLRpc holds the XML-RPC information.
type XMLRpc struct {
	host string
	port int
}

// New creates a new XMLRpc object for communicating with XML-RPC server.
func New(host string, port int) *XMLRpc {
	return &XMLRpc{host: host, port: port}
}

// Call represents a XML-RPC call request.
type Call struct {
	method  string
	args    []interface{}
	timeout time.Duration
}

// methodCall mirrors the structure of an XML-RPC method call.
type methodCall struct {
	XMLName    xml.Name `xml:"methodCall"`
	MethodName string   `xml:"methodName"`
	Params     *[]param `xml:"params>param"`
}

// methodResponse is an XML-RPC response.
type methodResponse struct {
	XMLName xml.Name `xml:"methodResponse"`
	Params  *[]param `xml:"params>param,omitempty"`
	Fault   *fault   `xml:"fault,omitempty"`
}

// param is an XML-RPC param.
type param struct {
	Value value `xml:"value"`
}

// fault is an XML-RPC fault.
// If present, it usually contains in its value a struct of two members:
// faultCode (an int) and faultString (a string).
type fault struct {
	Value value `xml:"value"`
}

// value is an XML-RPC value.
type value struct {
	Boolean *string    `xml:"boolean,omitempty"`
	Double  *string    `xml:"double,omitempty"`
	Int     *string    `xml:"int,omitempty"`
	Str     *string    `xml:"string,omitempty"`
	Base64  *string    `xml:"base64,omitempty"`
	Array   *xmlArray  `xml:"array,omitempty"`
	Struct  *xmlStruct `xml:"struct,omitempty"`
}

// xmlArray is an XML-RPC array.
type xmlArray struct {
	Values []value `xml:"data>value,omitempty"`
}

// xmlStruct is an XML-RPC struct.
type xmlStruct struct {
	Members []member `xml:"member,omitempty"`
}

// member is an XML-RPC object containing a name and a value.
type member struct {
	Name  string `xml:"name"`
	Value value  `xml:"value"`
}

// String implements the String() interface of value.
func (v value) String() string {
	if v.Boolean != nil {
		return "(boolean)" + *v.Boolean
	}
	if v.Double != nil {
		return "(double)" + *v.Double
	}
	if v.Int != nil {
		return "(int)" + *v.Int
	}
	if v.Str != nil {
		return "(string)" + *v.Str
	}
	if v.Array != nil {
		var values []string
		for _, e := range v.Array.Values {
			values = append(values, e.String())
		}
		return "[" + strings.Join(values, ", ") + "]"
	}
	if v.Struct != nil {
		var values []string
		for _, m := range v.Struct.Members {
			values = append(values, fmt.Sprintf("%s: %s", m.Name, m.Value.String()))
		}
		return "{" + strings.Join(values, ", ") + "}"
	}
	if v.Base64 != nil {
		return "(base64)" + *v.Base64
	}
	return "<empty>"
}

// FaultError is a type of error representing an XML-RPC fault.
type FaultError struct {
	Code   int
	Reason string

	// Including *errors.E allows FaultError to work with the Tast errors library.
	*errors.E
}

// NewFaultError creates a FaultError.
func NewFaultError(code int, reason string) FaultError {
	return FaultError{
		Code:   code,
		Reason: reason,
		E:      errors.Errorf("xml-rpc fault with code %d: %s", code, reason),
	}
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

// bytesToXMLBase64 converts a Go []byte to an XML-RPC base64-encoded string.
func bytesToXMLBase64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

// xmlBase64ToBytes converts an XML-RPC base64-encoded string to Go []byte.
func xmlBase64ToBytes(base64Str string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(base64Str)
}

// newValue creates an XML-RPC <value>.
func newValue(in interface{}) (value, error) {
	// TODO(crbug.com/1201727): Support more data types, such as Golang map to XML-RPC struct.
	inType := reflect.TypeOf(in)
	if (inType.Kind() == reflect.Slice || inType.Kind() == reflect.Array) &&
		inType.Elem().Kind() != reflect.Uint8 {
		// This is a slice or array, but not a []byte (aka a []uint8).
		v := reflect.ValueOf(in)
		var a xmlArray
		for i := 0; i < v.Len(); i++ {
			val, err := newValue(v.Index(i).Interface())
			if err != nil {
				return value{}, err
			}
			a.Values = append(a.Values, val)
		}
		return value{Array: &a}, nil
	}
	switch v := in.(type) {
	case string:
		s := v
		return value{Str: &s}, nil
	case bool:
		b := boolToXMLBoolean(v)
		return value{Boolean: &b}, nil
	case int:
		i, err := intToXMLInteger(v)
		if err != nil {
			return value{}, err
		}
		return value{Int: &i}, nil
	case float64:
		f := float64ToXMLDouble(v)
		return value{Double: &f}, nil
	case []byte:
		b := bytesToXMLBase64(v)
		return value{Base64: &b}, nil
	case map[string]string:
		var s xmlStruct
		for key, obj := range v {
			str := obj
			s.Members = append(s.Members, member{Name: key, Value: value{Str: &str}})
		}
		return value{Struct: &s}, nil
	case map[string]interface{}:
		var s xmlStruct
		for key, obj := range v {
			val, err := newValue(obj)
			if err != nil {
				return value{}, errors.Wrapf(err, "failed when calling newValue on key: %v value: %v", key, obj)
			}
			s.Members = append(s.Members, member{Name: key, Value: val})
		}
		return value{Struct: &s}, nil
	default:
		// This is to support type definition wrapping around primitive type
		// without having the client code perform a type conversion.
		// e.g. type NewType int
		// If the underlying type is one of the primitive types supported,
		// we recursively call NewValue by peeling of one layer of indirection.
		switch reflect.TypeOf(in).Kind() {
		case reflect.Int:
			return newValue(int(reflect.ValueOf(in).Int()))
		case reflect.Bool:
			return newValue(reflect.ValueOf(in).Bool())
		case reflect.Float64:
			return newValue(float64(reflect.ValueOf(in).Float()))
		case reflect.String:
			return newValue(reflect.ValueOf(in).String())
		}
	}
	return value{}, errors.Errorf("%q is not a supported type for newValue", reflect.TypeOf(in))
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

// NewCall creates a XML-RPC call.
func NewCall(method string, args ...interface{}) Call {
	return Call{method, args, defaultRPCTimeout}
}

// NewCallTimeout creates a XML-RPC call.
func NewCallTimeout(method string, timeout time.Duration, args ...interface{}) Call {
	return Call{method, args, timeout}
}

// serializeMethodCall turns a method and args into a serialized XML-RPC method call.
func serializeMethodCall(cl Call) ([]byte, error) {
	params, err := newParams(cl.args)
	if err != nil {
		return nil, err
	}
	return xml.Marshal(&methodCall{MethodName: cl.method, Params: &params})
}

// getTimeout returns the lowest of the default timeout or remaining duration
// to the context's deadline.
func getTimeout(ctx context.Context, cl Call) time.Duration {
	timeout := cl.timeout
	if dl, ok := ctx.Deadline(); ok {
		newTimeout := dl.Sub(time.Now())
		if newTimeout < timeout {
			timeout = newTimeout
		}
	}
	return timeout
}

// unpackValue unpacks a value struct into the given pointers.
func unpackValue(val value, out interface{}) error {
	//TODO(crbug.com/1201727): Support unpack more data types, such as XML-RPC struct.
	switch o := out.(type) {
	case *string:
		if val.Str == nil {
			return errors.Errorf("value %s is not a string value", val)
		}
		*o = *val.Str
	case *bool:
		if val.Boolean == nil {
			return errors.Errorf("value %s is not a boolean value", val)
		}
		v, err := xmlBooleanToBool(*val.Boolean)
		if err != nil {
			return err
		}
		*o = v
	case *int:
		if val.Int == nil {
			return errors.Errorf("value %s is not an int value", val)
		}
		i, err := xmlIntegerToInt(*val.Int)
		if err != nil {
			return err
		}
		*o = i
	case *float64:
		if val.Double == nil {
			return errors.Errorf("value %s is not a double value", val)
		}
		f, err := xmlDoubleToFloat64(*val.Double)
		if err != nil {
			return err
		}
		*o = f
	case *[]string:
		if val.Array == nil {
			return errors.Errorf("value %s is not an array value", val)
		}
		for _, e := range val.Array.Values {
			var i string
			if err := unpackValue(e, &i); err != nil {
				return err
			}
			*o = append(*o, i)
		}
	case *[]bool:
		if val.Array == nil {
			return errors.Errorf("value %s is not an array value", val)
		}
		for _, e := range val.Array.Values {
			var i bool
			if err := unpackValue(e, &i); err != nil {
				return err
			}
			*o = append(*o, i)
		}
	case *[]int:
		if val.Array == nil {
			return errors.Errorf("value %s is not an array value", val)
		}
		for _, e := range val.Array.Values {
			var i int
			if err := unpackValue(e, &i); err != nil {
				return err
			}
			*o = append(*o, i)
		}
	case *[]float64:
		if val.Array == nil {
			return errors.Errorf("value %s is not an array value", val)
		}
		for _, e := range val.Array.Values {
			var i float64
			if err := unpackValue(e, &i); err != nil {
				return err
			}
			*o = append(*o, i)
		}
	case *[]byte:
		if val.Base64 == nil {
			return errors.Errorf("value %s is not a []byte (read as a base64 encoded string) value", val)
		}
		i, err := xmlBase64ToBytes(*val.Base64)
		if err != nil {
			return err
		}
		*o = i
	case *map[string]string:
		if val.Struct == nil {
			return errors.Errorf("value %s is not a map", val)
		}
		for _, e := range val.Struct.Members {
			var i string
			if err := unpackValue(e.Value, &i); err != nil {
				return err
			}
			(*o)[e.Name] = i
		}
	case *[][]string:
		if val.Array == nil {
			return errors.Errorf("value %s is not an array value", val)
		}
		for _, e := range val.Array.Values {
			var value []string
			if err := unpackValue(e, &value); err != nil {
				return err
			}
			*o = append(*o, value)
		}
	case *[]interface{}:
		values := val.Array.Values
		*o = make([]interface{}, 0)
		for _, eVal := range values {
			if eVal.Array != nil {
				var unpackedValue []interface{}
				if err := unpackValue(eVal, &unpackedValue); err != nil {
					return err
				}
				*o = append(*o, unpackedValue)
			} else if eVal.Struct != nil {
				var unpackedValue map[string]interface{}
				if err := unpackValue(eVal, &unpackedValue); err != nil {
					return err
				}
				*o = append(*o, unpackedValue)
			} else if eVal.Str != nil {
				var unpackedValue string
				if err := unpackValue(eVal, &unpackedValue); err != nil {
					return err
				}
				*o = append(*o, unpackedValue)
			} else if eVal.Int != nil {
				var unpackedValue int
				if err := unpackValue(eVal, &unpackedValue); err != nil {
					return err
				}
				*o = append(*o, unpackedValue)
			} else if eVal.Double != nil {
				var unpackedValue float64
				if err := unpackValue(eVal, &unpackedValue); err != nil {
					return err
				}
				*o = append(*o, unpackedValue)
			} else if eVal.Boolean != nil {
				var unpackedValue bool
				if err := unpackValue(eVal, &unpackedValue); err != nil {
					return err
				}
				*o = append(*o, unpackedValue)
			} else {
				return errors.Errorf("unable to deduce type of struct value %q", eVal.String())
			}
		}
	case *map[string]interface{}:
		if val.Struct == nil {
			return errors.Errorf("value %s is not a map", val)
		}
		for _, e := range val.Struct.Members {
			eVal := e.Value
			if eVal.Array != nil {
				var unpackedValue []interface{}
				if err := unpackValue(eVal, &unpackedValue); err != nil {
					return err
				}
				(*o)[e.Name] = unpackedValue
			} else if eVal.Struct != nil {
				var unpackedValue map[string]interface{}
				if err := unpackValue(eVal, &unpackedValue); err != nil {
					return err
				}
				(*o)[e.Name] = unpackedValue
			} else if eVal.Str != nil {
				var unpackedValue string
				if err := unpackValue(eVal, &unpackedValue); err != nil {
					return err
				}
				(*o)[e.Name] = unpackedValue
			} else if eVal.Int != nil {
				var unpackedValue int
				if err := unpackValue(eVal, &unpackedValue); err != nil {
					return err
				}
				(*o)[e.Name] = unpackedValue
			} else if eVal.Double != nil {
				var unpackedValue float64
				if err := unpackValue(eVal, &unpackedValue); err != nil {
					return err
				}
				(*o)[e.Name] = unpackedValue
			} else if eVal.Boolean != nil {
				var unpackedValue bool
				if err := unpackValue(eVal, &unpackedValue); err != nil {
					return err
				}
				(*o)[e.Name] = unpackedValue
			} else {
				return errors.Errorf("unable to deduce type of struct value %q", eVal.String())
			}
		}
	case *interface{}:

	default:
		return errors.Errorf("%q is not a supported type for unpackValue", reflect.TypeOf(out))
	}
	return nil
}

// unpack extracts a response's arguments into a list of given pointers.
func (r *methodResponse) unpack(out []interface{}) error {
	if r.Params == nil {
		if len(out) != 0 {
			return errors.Errorf("response contains no args; want %d", len(out))
		}
		return nil
	}
	if len(*r.Params) != len(out) {
		return errors.Errorf("response contains %d arg(s); want %d", len(*r.Params), len(out))
	}

	for i, p := range *r.Params {
		if err := unpackValue(p.Value, out[i]); err != nil {
			return errors.Wrapf(err, "failed to unpack response param at index %d", i)
		}
	}

	return nil
}

// checkFault returns a FaultError if the response contains a fault with a non-zero faultCode.
func (r *methodResponse) checkFault() error {
	if r.Fault == nil {
		return nil
	}
	if r.Fault.Value.Struct == nil {
		return errors.Errorf("fault %s doesn't contain xml-rpc struct", r.Fault.Value)
	}
	var rawFaultCode string
	var faultString string
	for _, m := range r.Fault.Value.Struct.Members {
		switch m.Name {
		case "faultCode":
			if m.Value.Int == nil {
				return errors.Errorf("faultCode %s doesn't provide integer value", m.Value)
			}
			rawFaultCode = *m.Value.Int
		case "faultString":
			if m.Value.Str == nil {
				return errors.Errorf("faultString %s doesn't provide string value", m.Value)
			}
			faultString = *m.Value.Str
		default:
			return errors.Errorf("unexpected fault member name: %s", m.Name)
		}
	}
	faultCode, err := xmlIntegerToInt(rawFaultCode)
	if err != nil {
		return errors.Wrap(err, "interpreting fault code")
	}
	if faultCode == 0 {
		return errors.Errorf("response contained a fault with unexpected code 0; want non-0: faultString=%s", faultString)
	}
	return NewFaultError(faultCode, faultString)
}

// Run makes an XML-RPC call to the server.
func (r *XMLRpc) Run(ctx context.Context, cl Call, out ...interface{}) error {
	body, err := serializeMethodCall(cl)
	if err != nil {
		return err
	}

	// Get RPC timeout duration from context or use default.
	timeout := getTimeout(ctx, cl)
	serverURL := fmt.Sprintf("http://%s:%d", r.host, r.port)
	httpClient := &http.Client{Timeout: timeout}

	resp, err := httpClient.Post(serverURL, "text/xml", bytes.NewBuffer(body))
	if err != nil {
		return errors.Wrapf(err, "timeout = %v", timeout)
	}
	defer resp.Body.Close()

	// Read body and unmarshal XML.
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	res := methodResponse{}
	if err = xml.Unmarshal(bodyBytes, &res); err != nil {
		return err
	}
	if err = res.checkFault(); err != nil {
		return err
	}

	// If outs are specified, unpack response params.
	// Otherwise, return without unpacking.
	if len(out) > 0 {
		if err := res.unpack(out); err != nil {
			testing.ContextLogf(ctx, "Failed to unpack XML-RPC response for request %v: %s : err: %v", cl, string(bodyBytes), err)
			return err
		}
	}
	return nil
}

// CallBuilder creates a new CallBuilder instance that uses this XMLRpc client.
func (r *XMLRpc) CallBuilder() *CallBuilder {
	return NewCallBuilder(r)
}
