// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package xmlrpc

import (
	"encoding/xml"
	"math"
	"strconv"
	"testing"
)

func TestXMLBooleanToBool(t *testing.T) {
	for _, tc := range []struct {
		input     string
		expected  bool
		expectErr bool
	}{
		{"1", true, false},
		{"0", false, false},
		{"rutabaga", false, true},
	} {
		actual, err := xmlBooleanToBool(tc.input)
		if tc.expectErr {
			if err == nil {
				t.Errorf("xmlBooleanToBool(%q) unexpectedly succeeded", tc.input)
			}
		} else {
			if err != nil {
				t.Errorf("xmlBooleanToBool(%q) failed: %v", tc.input, err)
			}
			if actual != tc.expected {
				t.Errorf("xmlBooleanToBool(%q) = %v; want %v", tc.input, actual, tc.expected)
			}
		}
	}
}

func TestBoolToXMLBoolean(t *testing.T) {
	for _, tc := range []struct {
		input    bool
		expected string
	}{
		{true, "1"},
		{false, "0"},
	} {
		actual := boolToXMLBoolean(tc.input)
		if actual != tc.expected {
			t.Errorf("boolToXMLBoolean(%v) = %q; want %q", tc.input, actual, tc.expected)
		}
	}
}

func TestXMLIntegerToInt(t *testing.T) {
	for _, tc := range []struct {
		input     string
		expected  int
		expectErr bool
	}{
		{"1", 1, false},
		{"0", 0, false},
		{"-1", -1, false},
		{"1.5", 0, true},
		{"3000000000", 0, true}, // too big for an int32
		{"easter-egg", 0, true},
		{"true", 0, true},
	} {
		actual, err := xmlIntegerToInt(tc.input)
		if tc.expectErr {
			if err == nil {
				t.Errorf("xmlIntegerToInt(%q) unexpectedly succeeded", tc.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("xmlIntegerToInt(%q) failed: %v", tc.input, err)
			continue
		}
		if actual != tc.expected {
			t.Errorf("xmlIntegerToInt(%q) = %v; want %v", tc.input, actual, tc.expected)
		}
	}
}

func TestIntToXMLInteger(t *testing.T) {
	for _, tc := range []struct {
		input     int
		expected  string
		expectErr bool
	}{
		{1, "1", false},
		{0, "0", false},
		{-1, "-1", false},
		{math.MaxInt64, "", true},
	} {
		actual, err := intToXMLInteger(tc.input)
		if tc.expectErr {
			if err == nil {
				t.Errorf("intToXMLInteger(%d) unexpectedly succeeded", tc.input)
				continue
			}
		}
		if actual != tc.expected {
			t.Errorf("IntToXMLInteger(%v) = %q; want %q", tc.input, actual, tc.expected)
		}
	}
}

func TestXMLDoubleToFloat64(t *testing.T) {
	for _, tc := range []struct {
		input     string
		expected  float64
		expectErr bool
	}{
		{"1", 1.0, false},
		{"3.14", 3.14, false},
		{"-3.14", -3.14, false},
		{strconv.FormatFloat(math.MaxFloat64, 'f', -1, 64), math.MaxFloat64, false},
		{strconv.FormatFloat(math.SmallestNonzeroFloat64, 'f', -1, 64), math.SmallestNonzeroFloat64, false},
		{"", 0.0, true},
		{"easter-egg", 0.0, true},
		{"true", 0.0, true},
	} {
		actual, err := xmlDoubleToFloat64(tc.input)
		if tc.expectErr {
			if err == nil {
				t.Errorf("xmlDoubleToFloat64(%q) unexpectedly succeeded", tc.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("xmlDoubleToFloat64(%q) failed: %v", tc.input, err)
			continue
		}
		if actual != tc.expected {
			t.Errorf("xmlDoubleToFloat64(%q) = %v; want %v", tc.input, actual, tc.expected)
		}
	}
}

func TestFloat64ToXMLDouble(t *testing.T) {
	for _, tc := range []struct {
		input    float64
		expected string
	}{
		{3.14, "3.14"},
		{0, "0"},
		{-1, "-1"},
	} {
		actual := float64ToXMLDouble(tc.input)
		if actual != tc.expected {
			t.Errorf("float64ToXMLDouble(%v) = %q; want %q", tc.input, actual, tc.expected)
		}
	}
}

func TestNewValue(t *testing.T) {
	expectedStr := "rutabaga"
	v, err := newValue(expectedStr)
	if err != nil {
		t.Errorf("newValue(%q) failed: %v", expectedStr, err)
		return
	}
	if v.String != expectedStr {
		t.Errorf("got %q; want %q", v.String, expectedStr)
	}

	expectedBool := true
	expectedBoolStr := "1"
	v, err = newValue(expectedBool)
	if err != nil {
		t.Errorf("input %v gave unexpected error: %v", expectedStr, err)
		return
	}
	if v.Boolean != expectedBoolStr {
		t.Errorf("got %q; want %q", v.Boolean, expectedBoolStr)
	}

	expectedInt := -1
	expectedIntStr := "-1"
	v, err = newValue(expectedInt)
	if err != nil {
		t.Errorf("input %v gave unexpected error: %v", expectedInt, err)
		return
	}
	if v.Int != expectedIntStr {
		t.Errorf("got %q; want %q", v.Int, expectedIntStr)
	}

	f := -3.14
	expectedDoubleStr := "-3.14"
	v, err = newValue(f)
	if err != nil {
		t.Errorf("input %f gave unexpected error: %v", f, err)
		return
	}
	if v.Double != expectedDoubleStr {
		t.Errorf("got %q; want %q", v.Double, expectedDoubleStr)
	}

	expectedUnsupported := struct{}{}
	v, err = newValue(expectedUnsupported)
	if err == nil {
		t.Errorf("input %v did not throw expected error", expectedUnsupported)
	}
}

func TestNewParams(t *testing.T) {
	actual, err := newParams([]interface{}{"rutabaga", true, -3.14})

	if err != nil {
		t.Errorf("got unexpected error: %v", err)
		return
	}
	if len(actual) != 3 {
		t.Errorf("got len %d; want %d", len(actual), 3)
	}
	if actual[0].Value.String != "rutabaga" {
		t.Errorf("for first return value got %q; want %q", actual[0].Value.String, "rutabaga")
	}
	if actual[1].Value.Boolean != "1" {
		t.Errorf("for second return value got %q; want %q", actual[1].Value.Boolean, "1")
	}
	if actual[2].Value.Double != "-3.14" {
		t.Errorf("for third return value got %q; want %q", actual[2].Value.Double, "-3.14")
	}
}

func TestUnpack(t *testing.T) {
	params, err := newParams([]interface{}{"rutabaga", true, 1, -3.14})
	if err != nil {
		t.Fatal("creating params: ", err)
	}
	resp := response{Params: params}
	var stringOut string
	var boolOut bool
	var intOut int
	var floatOut float64
	if err := resp.unpack([]interface{}{&stringOut, &boolOut, &intOut, &floatOut}); err != nil {
		t.Fatal("unpacking: ", err)
	}
	if stringOut != "rutabaga" {
		t.Errorf("unpacking %q: got %q; want %q", "rutabaga", stringOut, "rutabaga")
	}
	if boolOut != true {
		t.Errorf("unpacking %q: got %v; want %v", "true", boolOut, true)
	}
	if intOut != 1 {
		t.Errorf("unpacking %q: got %d; want %d", "1", intOut, 1)
	}
	if floatOut != -3.14 {
		t.Errorf("unpacking %q: got %f; want %f", "-3.14", floatOut, -3.14)
	}
}

func TestCheckFault(t *testing.T) {
	hasFault := []byte(`<?xml version='1.0'?>
	<methodResponse>
		<fault>
			<value>
				<struct>
					<member>
						<name>faultCode</name>
						<value>
							<int>1</int>
						</value>
					</member>
					<member>
						<name>faultString</name>
						<value>
							<string>Bad XML request!</string>
						</value>
					</member>
				</struct>
			</value>
		</fault>
	</methodResponse>`)
	noFault := []byte(`<?xml version='1.0'?>
	<methodResponse>
		<params>
			<param>
				<value>
					<string>Hello, world!</string>
				</value>
			</param>
		</params>
	</methodResponse>`)
	faultWithCode0 := []byte(`<?xml version='1.0'?>
	<methodResponse>
		<fault>
			<value>
				<struct>
					<member>
						<name>faultCode</name>
						<value>
							<int>0</int>
						</value>
					</member>
					<member>
						<name>faultString</name>
						<value>
							<string>Bad XML request!</string>
						</value>
					</member>
				</struct>
			</value>
		</fault>
	</methodResponse>`)
	faultWithUnexpectedMember := []byte(`<?xml version='1.0'?>
	<methodResponse>
		<fault>
			<value>
				<struct>
					<member>
						<name>faultCode</name>
						<value>
							<int>1</int>
						</value>
					</member>
					<member>
						<name>faultString</name>
						<value>
							<string>Bad XML request!</string>
						</value>
					</member>
					<member>
						<name>unexpectedMember</name>
						<value>
							<string>Foo</string>
						</value>
					</member>
				</struct>
			</value>
		</fault>
	</methodResponse>`)
	type testCase struct {
		b                   []byte
		expectErr           bool
		expectFault         bool
		expectedFaultCode   int
		expectedFaultString string
	}
	for i, tc := range []testCase{
		testCase{hasFault, true, true, 1, "Bad XML request!"},
		testCase{noFault, false, false, 0, ""},
		testCase{faultWithCode0, true, false, 0, ""},
		testCase{faultWithUnexpectedMember, true, false, 0, ""},
	} {
		// Check the XML bytes for fault.
		r := response{}
		if err := xml.Unmarshal(tc.b, &r); err != nil {
			t.Errorf("tc #%d: failed to unmarshal bytes: %v", i, err)
			continue
		}
		e := r.checkFault()

		// Check whether we got an error if expected.
		// If no error is expected, then continue early, since the rest of the test operates on the error.
		if !tc.expectErr {
			if e != nil {
				t.Errorf("tc #%d: unexpected checkFault error response: got %v; want nil", i, e)
			}
			continue
		} else if e == nil {
			t.Errorf("tc #%d: unexpected checkFault error response: got nil; want error", i)
			continue
		}

		// Check whether we got a fault if expected.
		// If no fault is expected, then continue early, since the rest of the test operates on the error.
		fe, isFault := e.(FaultError)
		if !tc.expectFault {
			if isFault {
				t.Errorf("tc #%d: unexpected checkFault error: got '%v'; want non-Fault error", i, e)
			}
			continue
		} else if !isFault {
			t.Errorf("tc #%d: unexpected checkFault error: got '%v'; want FaultError", i, e)
		}

		// Check the Fault's attributes
		if fe.Code != tc.expectedFaultCode {
			t.Errorf("tc #%d: checkFault returned unexpected code: got %d; want %d", i, fe.Code, tc.expectedFaultCode)
		}
		if fe.Reason != tc.expectedFaultString {
			t.Errorf("tc #%d: checkFault returned unexpected reason: got %s; want %s", i, fe.Reason, tc.expectedFaultString)
		}
	}
}
