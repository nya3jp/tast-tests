// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package nodewith is used to generate queries to find chrome.automation nodes.
package nodewith

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ui2/checked"
	"chromiumos/tast/local/chrome/ui2/restriction"
	"chromiumos/tast/local/chrome/ui2/role"
	"chromiumos/tast/local/chrome/ui2/state"
)

// Finder is a mapping of chrome.automation.FindParams to Golang with a nicer API.
// As defined in chromium/src/extensions/common/api/automation.idl
type Finder struct {
	ancestor   *Finder
	attributes map[string]interface{}
	first      bool
	role       role.Role
	state      map[state.State]bool
}

func newFinder() *Finder {
	return &Finder{
		attributes: make(map[string]interface{}),
		state:      make(map[state.State]bool),
	}
}

func (f *Finder) copy() *Finder {
	copy := newFinder()
	copy.ancestor = f.ancestor
	copy.role = f.role
	for k, v := range f.attributes {
		copy.attributes[k] = v
	}
	for k, v := range f.state {
		copy.state[k] = v
	}
	return copy
}

func (f *Finder) attributeBytes() ([]byte, error) {
	// json.Marshal can't be used because this is JavaScript code with regular expressions, not JSON.
	var buf bytes.Buffer
	buf.WriteByte('{')
	first := true
	for k, v := range f.attributes {
		if first {
			first = false
		} else {
			buf.WriteByte(',')
		}
		switch v := v.(type) {
		case string, checked.Checked, restriction.Restriction:
			fmt.Fprintf(&buf, "%q:%q", k, v)
		case int, float32, float64, bool:
			fmt.Fprintf(&buf, "%q:%v", k, v)
		case regexp.Regexp, *regexp.Regexp:
			fmt.Fprintf(&buf, `%q:/%v/`, k, v)
		default:
			return nil, errors.Errorf("nodewith.Finder does not support type(%T) for parameter(%s)", v, k)
		}
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func (f *Finder) bytes() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	attributes, err := f.attributeBytes()
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(&buf, `"attributes":%s,`, attributes)

	if f.role != "" {
		fmt.Fprintf(&buf, `"role":%q,`, f.role)
	}

	state, err := json.Marshal(f.state)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(&buf, `"state":%s`, state)

	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// FindQuery generates the JS query to find this node.
// It expects a variable "node" to already be defined in previous JS.
// It will write its output to "node" to store enable chaining.
func (f *Finder) FindQuery() (string, error) {
	var out string
	if f.ancestor != nil {
		q, err := f.ancestor.FindQuery()
		if err != nil {
			return "", errors.Wrap(err, "failed to convert ancestor query")
		}
		out += q
	}
	bytes, err := f.bytes()
	if err != nil {
		return "", errors.Wrapf(err, "failed to convert finder(%+v) to bytes", f)
	}
	if f.first {
		out += fmt.Sprintf(`
			node = node.find(%[1]s);
			if (!node) {
				throw 'failed to find node from query: %[1]s';
			}
		`, bytes)
	} else {
		out += fmt.Sprintf(`
			nodes = node.findAll(%[1]s);
			if (nodes.length == 0) {
				throw 'failed to find any nodes from query: %[1]s';
			} else if (nodes.length > 1) {
				throw 'node finder too generic: found multiple nodes from query: %[1]s';
			}
			node = nodes[0];
		`, bytes)
	}
	return out, nil
}

// Ancestor creates a Finder with the specified ancestor.
func Ancestor(a *Finder) *Finder {
	f := newFinder()
	f.ancestor = a
	return f
}

// Ancestor creates a copy of the input Finder with the specified ancestor.
func (f *Finder) Ancestor(a *Finder) *Finder {
	c := f.copy()
	c.ancestor = a
	return c
}

// Attribute creates a Finder with the specified attribute.
func Attribute(k string, v interface{}) *Finder {
	f := newFinder()
	f.attributes[k] = v
	return f
}

// Attribute creates a copy of the input Finder with the specified attribute.
func (f *Finder) Attribute(k string, v interface{}) *Finder {
	c := f.copy()
	c.attributes[k] = v
	return c
}

// First creates a Finder that will find the first node instead of requiring uniqueness.
func First() *Finder {
	f := newFinder()
	f.first = true
	return f
}

// First creates a a copy of the input Finder that will find the first node instead of requiring uniqueness.
func (f *Finder) First() *Finder {
	c := f.copy()
	c.first = true
	return c
}

// Role creates a Finder with the specified role.
func Role(r role.Role) *Finder {
	f := newFinder()
	f.role = r
	return f
}

// Role creates a copy of the input Finder with the specified role.
func (f *Finder) Role(r role.Role) *Finder {
	c := f.copy()
	c.role = r
	return c
}

// State creates a Finder with the specified state.
func State(k state.State, v bool) *Finder {
	f := newFinder()
	f.state[k] = v
	return f
}

// State creates a copy of the input Finder with the specified state.
func (f *Finder) State(k state.State, v bool) *Finder {
	c := f.copy()
	c.state[k] = v
	return c
}

// Name creates a Finder with the specified name.
func Name(n string) *Finder {
	return Attribute("name", n)
}

// Name creates a copy of the input Finder with the specified name.
func (f *Finder) Name(n string) *Finder {
	return f.Attribute("name", n)
}

// ClassName creates a Finder with the specified class name.
func ClassName(n string) *Finder {
	return Attribute("className", n)
}

// ClassName creates a copy of the input Finder with the specified class name.
func (f *Finder) ClassName(n string) *Finder {
	return f.Attribute("className", n)
}
