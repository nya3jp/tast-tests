// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package localstate provides utilities for accessing the browser's Local
// State file.
package localstate

import (
	"encoding/json"
	"io/ioutil"
	"os"
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
// pref name could have format "foo.bar.baz"
func MarshalPref(bt browser.Type, pref string, val interface{}) error {
	if pref == "" {
		return errors.Wrap(nil, "pref name cannot be empty")
	}
	var localStatePath = localStatePathAsh
	if bt == browser.TypeLacros {
		localStatePath = localStatePathLacros
	}
	var localState interface{}
	if _, err := os.Stat(localStatePath); err == nil {
		// localStatePath exists
		if err := Unmarshal(bt, &localState); err != nil {
			return errors.Wrap(err, "failed to retrieve Local State contents")
		}
	} else if errors.Is(err, os.ErrNotExist) {
		// localStatePath not exists
		// Note by reviewer: it is ok to create a file, if not exist.
		localState = make(map[string]interface{})
	} else {
		return errors.Wrap(err, "failed to retrieve Local State contents")
	}
	dict, ok := localState.(map[string]interface{})
	if !ok {
		return errors.Wrap(nil, "localState has an unexpected value type")
	}
	keys := strings.Split(pref, ".")
	for _, key := range keys[:len(keys)-1] {
		// loop up the map to see if the corresponding entry exists.
		e, ok := dict[key]
		if !ok {
			e = make(map[string]interface{})
			dict[key] = e
		}
		// Making sure the entry is a dict.
		dict, ok = e.(map[string]interface{})
		if !ok {
			return errors.Wrapf(nil, "failed to read Local State content %s", key)
		}
	}
	dict[keys[len(keys)-1]] = val
	if err := Marshal(bt, localState); err != nil {
		return errors.Wrap(err, "failed to wirte Local State contents")
	}
	return nil
}
