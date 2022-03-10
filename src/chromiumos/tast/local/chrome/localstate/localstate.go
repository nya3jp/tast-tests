// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package localstate provides utilities for accessing the browser's Local
// State file.
package localstate

import (
	"encoding/json"
	"io/ioutil"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/browser"
)

const (
	localStatePathAsh    = "/home/chronos/Local State"
	localStatePathLacros = "/home/chronos/user/lacros/Local State"
)

// Unmarshal performs json.Unmarshal on the contents of the browser's Local
// State file.
func Unmarshal(bt browser.Type, out interface{}) error {
	var localStatePath = localStatePathAsh
	if bt == browser.TypeLacros {
		localStatePath = localStatePathLacros
	}
	b, err := ioutil.ReadFile(localStatePath)
	if err != nil {
		return errors.Wrap(err, "failed to read Local State file")
	}
	if err := json.Unmarshal(b, out); err != nil {
		return errors.Wrap(err, "failed to unmarshal Local State")
	}
	return nil
}

// UnmarshalPref returns the unmarshaled value of a preference from the
// browser's Local State file. The preference name is a string such as
// "foo.bar.baz".
func UnmarshalPref(bt browser.Type, pref string) (interface{}, error) {
	path := strings.Split(pref, ".")
	var localState interface{}
	if err := Unmarshal(bt, &localState); err != nil {
		return nil, errors.Wrap(err, "failed to retrieve Local State contents")
	}
	for i, key := range path {
		unexpectedValueError := func() error {
			errPref := strings.Join(path[:i], ".")
			return errors.Errorf("unexpected value in Local State at %s: %v", errPref, localState)
		}
		dict, ok := localState.(map[string]interface{})
		if !ok {
			return nil, unexpectedValueError()
		}
		value, ok := dict[key]
		if !ok {
			return nil, unexpectedValueError()
		}
		localState = value
	}
	return localState, nil
}

// Marshal will update Local State with localState
// localState includes pref names and pref values that will be written to local state
// pref name with dot format "foo.bar.baz" is not allowed in localState interface,
// localState could include nested map type {"foo":{"bar":{"baz": 1234}}}
func Marshal(bt browser.Type, localState interface{}) error {
	path := localStatePathAsh
	if bt == browser.TypeLacros {
		path = localStatePathLacros
	}
	s, err := json.Marshal(localState)
	if err != nil {
		return errors.Wrap(err, "failed to marshal Local State")
	}
	if err := ioutil.WriteFile(path, s, 0644); err != nil {
		return errors.Wrap(err, "failed to write Local State")
	}
	return nil
}

// MarshalPref will update the pref with val in local state
// if pref has format "foo.bar.baz", it will be written as {"foo":{"bar":{"baz": 1234}}}
func MarshalPref(bt browser.Type, pref string, val interface{}) error {
	var localState interface{}
	if err := Unmarshal(bt, &localState); err != nil {
		return errors.Wrap(err, "failed to retrieve Local State contents")
	}
	localStateMap, ok := localState.(map[string]interface{})
	if !ok {
		return errors.Wrap(nil, "localState has an unexpected value type")
	}
	path := strings.Split(pref, ".")
	var newPref string
	var newValue interface{} = val
	for i := len(path) - 1; i > 0; i-- {
		newPref = path[i]
		dict := make(map[string]interface{})
		dict[newPref] = newValue
		newValue = dict
	}
	newPref = path[0]
	localStateMap[newPref] = newValue
	if err := Marshal(bt, localStateMap); err != nil {
		return errors.Wrap(err, "failed to wirte Local State contents")
	}
	return nil
}
