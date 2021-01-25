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
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
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

// newFinder returns a new Finder with an initialized attributes and state map.
// Other parameters are still set to default values.
func newFinder() *Finder {
	return &Finder{
		attributes: make(map[string]interface{}),
		state:      make(map[state.State]bool),
	}
}

// copy returns a copy of the input Finder.
// It copies all of the keys/values in attributes and state individually.
func (f *Finder) copy() *Finder {
	copy := newFinder()
	copy.ancestor = f.ancestor
	copy.first = f.first
	copy.role = f.role
	for k, v := range f.attributes {
		copy.attributes[k] = v
	}
	for k, v := range f.state {
		copy.state[k] = v
	}
	return copy
}

// attributesBytes returns the attributes map converted into json like bytes.
// json.Marshal can't be used because this is JavaScript code with regular expressions, not JSON.
func (f *Finder) attributesBytes() ([]byte, error) {
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

// bytes returns the input finder as bytes in the form of chrome.automation.FindParams.
func (f *Finder) bytes() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	attributes, err := f.attributesBytes()
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

// These are possible errors return by query for a node in JS.
// They are strings because JS does not return nice Go errors.
// Instead, it is simplest to just use strings.Contains with these errors.
const (
	ErrNotFound   = "failed to find node from query"
	ErrTooGeneric = "multiple nodes matched query, if you expect this and only want the first use First()"
)

// GenerateQuery generates the JS query to find this node.
// It must be called in an async function because it starts by awaiting the chrome.automation Desktop node.
// The final node will be in the variable node.
func (f *Finder) GenerateQuery() (string, error) {
	// Both node and nodes need to be generated now so they can be used in the subqueries.
	out := `
		let node = await tast.promisify(chrome.automation.getDesktop)();
		let nodes = [];
	`
	subQuery, err := f.generateSubQuery()
	if err != nil {
		return "", err
	}
	return out + subQuery, nil
}

// generateSubQuery is a helper function for GenerateQuery.
// It creates the JS query to find a node without awaiting the Desktop node.
func (f *Finder) generateSubQuery() (string, error) {
	var out string
	if f.ancestor != nil {
		q, err := f.ancestor.generateSubQuery()
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
				throw '%[2]s: %[1]s';
			}
		`, bytes, ErrNotFound)
	} else {
		out += fmt.Sprintf(`
			nodes = node.findAll(%[1]s);
			if (nodes.length == 0) {
				throw '%[2]s: %[1]s';
			} else if (nodes.length > 1) {
				throw '%[3]s: %[1]s';
			}
			node = nodes[0];
		`, bytes, ErrNotFound, ErrTooGeneric)
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

// NameContaining creates a Finder with a name containing the specified string.
func NameContaining(n string) *Finder {
	r := regexp.MustCompile(fmt.Sprintf(".*%s.*", regexp.QuoteMeta(n)))
	return Attribute("name", r)
}

// NameContaining creates a copy of the input Finder with a name containing the specified string.
func (f *Finder) NameContaining(n string) *Finder {
	r := regexp.MustCompile(fmt.Sprintf(".*%s.*", regexp.QuoteMeta(n)))
	return f.Attribute("name", r)
}

// NameRegex creates a Finder with a name containing the specified regexp.
func NameRegex(r *regexp.Regexp) *Finder {
	return Attribute("name", r)
}

// NameRegex creates a copy of the input Finder with a name containing the specified regexp.
func (f *Finder) NameRegex(r *regexp.Regexp) *Finder {
	return f.Attribute("name", r)
}

// ClassName creates a Finder with the specified class name.
func ClassName(n string) *Finder {
	return Attribute("className", n)
}

// ClassName creates a copy of the input Finder with the specified class name.
func (f *Finder) ClassName(n string) *Finder {
	return f.Attribute("className", n)
}

// AutofillAvailable creates a Finder with AutofillAvailable set to true.
func AutofillAvailable() *Finder {
	return State(state.AutofillAvailable, true)
}

// AutofillAvailable creates a copy of the input Finder with AutofillAvailable set to true.
func (f *Finder) AutofillAvailable() *Finder {
	return f.State(state.AutofillAvailable, true)
}

// Collapsed creates a Finder with Collapsed set to true.
func Collapsed() *Finder {
	return State(state.Collapsed, true)
}

// Collapsed creates a copy of the input Finder with Collapsed set to true.
func (f *Finder) Collapsed() *Finder {
	return f.State(state.Collapsed, true)
}

// Default creates a Finder with Default set to true.
func Default() *Finder {
	return State(state.Default, true)
}

// Default creates a copy of the input Finder with Default set to true.
func (f *Finder) Default() *Finder {
	return f.State(state.Default, true)
}

// Editable creates a Finder with Editable set to true.
func Editable() *Finder {
	return State(state.Editable, true)
}

// Editable creates a copy of the input Finder with Editable set to true.
func (f *Finder) Editable() *Finder {
	return f.State(state.Editable, true)
}

// Expanded creates a Finder with Expanded set to true.
func Expanded() *Finder {
	return State(state.Expanded, true)
}

// Expanded creates a copy of the input Finder with Expanded set to true.
func (f *Finder) Expanded() *Finder {
	return f.State(state.Expanded, true)
}

// Focusable creates a Finder with Focusable set to true.
func Focusable() *Finder {
	return State(state.Focusable, true)
}

// Focusable creates a copy of the input Finder with Focusable set to true.
func (f *Finder) Focusable() *Finder {
	return f.State(state.Focusable, true)
}

// Focused creates a Finder with Focused set to true.
func Focused() *Finder {
	return State(state.Focused, true)
}

// Focused creates a copy of the input Finder with Focused set to true.
func (f *Finder) Focused() *Finder {
	return f.State(state.Focused, true)
}

// Horizontal creates a Finder with Horizontal set to true.
func Horizontal() *Finder {
	return State(state.Horizontal, true)
}

// Horizontal creates a copy of the input Finder with Horizontal set to true.
func (f *Finder) Horizontal() *Finder {
	return f.State(state.Horizontal, true)
}

// Hovered creates a Finder with Hovered set to true.
func Hovered() *Finder {
	return State(state.Hovered, true)
}

// Hovered creates a copy of the input Finder with Hovered set to true.
func (f *Finder) Hovered() *Finder {
	return f.State(state.Hovered, true)
}

// Ignored creates a Finder with Ignored set to true.
func Ignored() *Finder {
	return State(state.Ignored, true)
}

// Ignored creates a copy of the input Finder with Ignored set to true.
func (f *Finder) Ignored() *Finder {
	return f.State(state.Ignored, true)
}

// Invisible creates a Finder with Invisible set to true.
func Invisible() *Finder {
	return State(state.Invisible, true)
}

// Invisible creates a copy of the input Finder with Invisible set to true.
func (f *Finder) Invisible() *Finder {
	return f.State(state.Invisible, true)
}

// Linked creates a Finder with Linked set to true.
func Linked() *Finder {
	return State(state.Linked, true)
}

// Linked creates a copy of the input Finder with Linked set to true.
func (f *Finder) Linked() *Finder {
	return f.State(state.Linked, true)
}

// Multiline creates a Finder with Multiline set to true.
func Multiline() *Finder {
	return State(state.Multiline, true)
}

// Multiline creates a copy of the input Finder with Multiline set to true.
func (f *Finder) Multiline() *Finder {
	return f.State(state.Multiline, true)
}

// Multiselectable creates a Finder with Multiselectable set to true.
func Multiselectable() *Finder {
	return State(state.Multiselectable, true)
}

// Multiselectable creates a copy of the input Finder with Multiselectable set to true.
func (f *Finder) Multiselectable() *Finder {
	return f.State(state.Multiselectable, true)
}

// Offscreen creates a Finder with Offscreen set to true.
func Offscreen() *Finder {
	return State(state.Offscreen, true)
}

// Offscreen creates a copy of the input Finder with Offscreen set to true.
func (f *Finder) Offscreen() *Finder {
	return f.State(state.Offscreen, true)
}

// Protected creates a Finder with Protected set to true.
func Protected() *Finder {
	return State(state.Protected, true)
}

// Protected creates a copy of the input Finder with Protected set to true.
func (f *Finder) Protected() *Finder {
	return f.State(state.Protected, true)
}

// Required creates a Finder with Required set to true.
func Required() *Finder {
	return State(state.Required, true)
}

// Required creates a copy of the input Finder with Required set to true.
func (f *Finder) Required() *Finder {
	return f.State(state.Required, true)
}

// RichlyEditable creates a Finder with RichlyEditable set to true.
func RichlyEditable() *Finder {
	return State(state.RichlyEditable, true)
}

// RichlyEditable creates a copy of the input Finder with RichlyEditable set to true.
func (f *Finder) RichlyEditable() *Finder {
	return f.State(state.RichlyEditable, true)
}

// Vertical creates a Finder with Vertical set to true.
func Vertical() *Finder {
	return State(state.Vertical, true)
}

// Vertical creates a copy of the input Finder with Vertical set to true.
func (f *Finder) Vertical() *Finder {
	return f.State(state.Vertical, true)
}

// Visited creates a Finder with Visited set to true.
func Visited() *Finder {
	return State(state.Visited, true)
}

// Visited creates a copy of the input Finder with Visited set to true.
func (f *Finder) Visited() *Finder {
	return f.State(state.Visited, true)
}
