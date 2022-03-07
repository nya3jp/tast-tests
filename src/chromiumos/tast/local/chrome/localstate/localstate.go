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
		dict, ok := localState.(map[string]interface{})
		if !ok {
			errPref := strings.Join(path[:i], ".")
			return nil, errors.Errorf("unexpected value in Local State at %s: %v", errPref, localState)
		}
		localState = dict[key]
	}
	return localState, nil
}
