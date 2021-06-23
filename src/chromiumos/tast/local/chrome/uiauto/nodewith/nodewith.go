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
	"sort"
	"strings"

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
	// name contains a mapping from locale to a matcher for name for that language.
	// eg. {"en": helloregex, "en-AU": gdayregex, "fr": bonjourregex} would indicate:
	// * A device set to fr, fr-CA, or any french variant would attempt to match the bonjour regex.
	// * A device set to en-AU would attempt to match the gday regex.
	// * A device set to en, en-US, or any non-AU english variant would attempt to match the hello regex.
	name  map[string]regexp.Regexp
	first bool
	root  bool
	nth   int
	role  role.Role
	state map[state.State]bool
}

// newFinder returns a new Finder with an initialized attributes and state map.
// Other parameters are still set to default values.
func newFinder() *Finder {
	return &Finder{
		attributes: make(map[string]interface{}),
		state:      make(map[state.State]bool),
		name:       make(map[string]regexp.Regexp),
	}
}

// copy returns a copy of the input Finder.
// It copies all of the keys/values in attributes and state individually.
func (f *Finder) copy() *Finder {
	copy := newFinder()
	copy.ancestor = f.ancestor
	copy.first = f.first
	copy.nth = f.nth
	copy.role = f.role
	for k, v := range f.attributes {
		copy.attributes[k] = v
	}
	for k, v := range f.state {
		copy.state[k] = v
	}
	for k, v := range f.name {
		copy.name[k] = v
	}
	return copy
}

// convertRegexp converts golang regexp to javascript format.
// The regular expressions used for looking up node is javascript code.
// The original regex provided in Golang needs to be converted.
// This function is not generally working on all situations.
func convertRegexp(goRegexp *regexp.Regexp) string {
	jsRegexStr := strings.ReplaceAll(fmt.Sprintf("%v", goRegexp), "/", "\\/")
	const ignoreCase = `(?i)`
	if strings.HasPrefix(jsRegexStr, ignoreCase) || strings.HasPrefix(jsRegexStr, "^"+ignoreCase) {
		return fmt.Sprintf("/%s/i", strings.Replace(jsRegexStr, ignoreCase, "", 1))
	}
	return fmt.Sprintf("/%s/", jsRegexStr)
}

// attributesBytes returns the attributes map converted into json like bytes.
// json.Marshal can't be used because this is JavaScript code with regular expressions, not JSON.
func (f *Finder) attributesBytes() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for k, v := range f.attributes {
		switch v := v.(type) {
		case string, checked.Checked, restriction.Restriction:
			fmt.Fprintf(&buf, "%q:%q,", k, v)
		case int, float32, float64, bool:
			fmt.Fprintf(&buf, "%q:%v,", k, v)
		case *regexp.Regexp:
			fmt.Fprintf(&buf, `%q:%s,`, k, convertRegexp(v))
		default:
			return nil, errors.Errorf("nodewith.Finder does not support type(%T) for parameter(%s)", v, k)
		}
	}
	if len(f.name) != 0 {
		// Sorting keys is required for consistency for unit tests.
		locales := make([]string, 0, len(f.name))
		for locale := range f.name {
			if !regexp.MustCompile("^[a-z][a-z](-[A-Z][A-Z])?$").Match([]byte(locale)) {
				return nil, errors.Errorf("nodewith.Finder name must match the regex [a-z][a-z](-[A-Z][A-Z])? (eg. \"en\" or \"en-US\") - got %s", locale)
			}
			locales = append(locales, locale)
		}
		sort.Strings(locales)
		fmt.Fprintf(&buf, "\"name\":selectName({")
		for _, locale := range locales {
			regex := f.name[locale]
			// We need to escape all "/", to avoid something like the regex "a/b" being translated to /a/b/,
			// which is invalid syntax in javascript.
			fmt.Fprintf(&buf, `%q:%s,`, locale, convertRegexp(&regex))
		}
		fmt.Fprintf(&buf, "})")
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
	ErrNotFound   = "failed to find node with properties"
	ErrTooGeneric = "multiple nodes matched, if you expect this and only want the first use First()"
)

// GenerateQuery generates the JS query to find this node.
// It must be called in an async function because it starts by awaiting the chrome.automation Desktop node.
// The final node will be in the variable node.
func (f *Finder) GenerateQuery() (string, error) {
	return f.generateQuery(false)
}

// GenerateQueryForMultipleNodes generates the JS query to find one or more nodes.
// It must be called in an async function because it starts by awaiting the chrome.automation Desktop node.
func (f *Finder) GenerateQueryForMultipleNodes() (string, error) {
	return f.generateQuery(true)
}

// generateQuery generates the tree of queries and then wraps it to give it access to variables it may need.
func (f *Finder) generateQuery(multipleNodes bool) (string, error) {
	// Both node and nodes need to be generated now so they can be used in the subqueries.
	out := `
		let node = await tast.promisify(chrome.automation.getDesktop)();
		let nodes = [];
	`
	if f.nameInTree() {
		out += `
		 let locale = chrome.i18n.getUILanguage();
		 function selectName(names) {
			 return names[locale] || names[locale.split("-")[0]] || names["en"]
		 }
		`
	}
	if f.root {
		return out, nil
	}
	subQuery, err := f.generateSubQuery(multipleNodes)
	if err != nil {
		return "", err
	}
	return out + subQuery, nil
}

// nameInTree returns whether this node or any of its sub-nodes have used the Name attribute.
func (f *Finder) nameInTree() bool {
	return len(f.name) != 0 || (f.ancestor != nil && f.ancestor.nameInTree())
}

// Pretty returns a nice-looking human-readable version of the finder.
// For example, Pretty(Name("hello").ClassName("cls").Ancestor(Role(role.Button)))
// will return `{name: /^hello$/, className: "cls", ancestor: {role: button}}`.
func (f *Finder) Pretty() string {
	var result []string
	if name, ok := f.name["en"]; ok {
		result = append(result, fmt.Sprintf("name: /%v/", &name))
	}
	for k, v := range f.attributes {
		switch v := v.(type) {
		case int, float32, float64, bool:
			result = append(result, fmt.Sprintf("%s: %v", k, v))
		case *regexp.Regexp:
			result = append(result, fmt.Sprintf("%s: /%v/", k, v))
		default:
			result = append(result, fmt.Sprintf("%s: %q", k, v))
		}
	}

	if f.role != "" {
		result = append(result, fmt.Sprintf("role: %s", f.role))
	}

	if len(f.state) != 0 {
		result = append(result, fmt.Sprintf("state: %v", f.state))
	}

	if f.first {
		result = append(result, "first: true")
	}

	if f.nth > 0 {
		result = append(result, fmt.Sprintf("nth: %d", f.nth))
	}

	if f.ancestor != nil {
		result = append(result, "ancestor: "+f.ancestor.Pretty())
	}
	return "{" + strings.Join(result, ", ") + "}"
}

// generateSubQuery is a helper function for GenerateQuery.
// It creates the JS query to find a node without awaiting the Desktop node.
func (f *Finder) generateSubQuery(multipleNodes bool) (string, error) {
	var out string
	if f.ancestor != nil {
		q, err := f.ancestor.generateSubQuery(false)
		if err != nil {
			return "", errors.Wrap(err, "failed to convert ancestor query")
		}
		out += q
	}
	bytes, err := f.bytes()
	if err != nil {
		return "", errors.Wrapf(err, "failed to convert finder(%+v) to bytes", f)
	}
	errNotFoundBytes, err := json.Marshal(ErrNotFound + ": " + f.Pretty())
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal not found error")
	}
	errTooGenericBytes, err := json.Marshal(ErrTooGeneric + ": " + f.Pretty())
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal too generic error")
	}
	if f.first {
		out += fmt.Sprintf(`
			node = node.find(%[1]s);
			if (!node) {
				throw %[2]q;
			}
		`, bytes, errNotFoundBytes)
	} else {
		out += fmt.Sprintf(`
			nodes = node.findAll(%[1]s);
		`, bytes)
		if !multipleNodes {
			out += fmt.Sprintf(`
			if (nodes.length <= %[2]d) {
				throw %[3]q;
			} else if (%[2]d == 0 && nodes.length > 1) {
				throw %[4]q;
			}
			node = nodes[%[2]d];
		`, bytes, f.nth, errNotFoundBytes, errTooGenericBytes)
		}
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

// FinalAncestor creates a copy of the chain of Finders such that the final ancestor is set to a.
// This can be used to scope an entire query to be a subset of different query.
func (f *Finder) FinalAncestor(a *Finder) *Finder {
	c := f.copy()
	tmp := c
	for tmp.ancestor != nil {
		tmp.ancestor = tmp.ancestor.copy()
		tmp = tmp.ancestor
	}
	tmp.ancestor = a
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

// Root creates a Finder that will find the root node.
func Root() *Finder {
	f := newFinder()
	f.root = true
	return f
}

// First creates a Finder that will find the first node instead of requiring uniqueness.
func First() *Finder {
	f := newFinder()
	f.first = true
	return f
}

// First creates a copy of the input Finder that will find the first node instead of requiring uniqueness.
func (f *Finder) First() *Finder {
	c := f.copy()
	c.first = true
	return c
}

// Nth creates a Finder that will find the n-th node in the matched nodes of the Finder, instead of requiring uniqueness.
func Nth(n int) *Finder {
	if n == 0 {
		return First()
	}
	f := newFinder()
	f.nth = n
	return f
}

// Nth creates a copy of the input Finder that will find the n-th node in the matched nodes of the Finder, instead of requiring uniqueness.
func (f *Finder) Nth(n int) *Finder {
	if n == 0 {
		return f.First()
	}
	c := f.copy()
	c.nth = n
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

// makeArXBString creates an approximation of the ar-XB string for a given string.
// If the string is incorrect, it can be overridden in Multilingual*.
func makeArXBString(english string) string {
	runes := []rune(english)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// makeEnXAString creates an approximation of the en-XA string for a given string.
// If the string is incorrect, it can be overridden in Multilingual*.
func makeEnXAString(english string, addNumbers bool) string {
	// Taken from tools/grit/grit/pseudolocales.py.
	numberWords := [...]string{"one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"}
	accentedLetters := map[rune]rune{
		'!': '\u00a1',
		'$': '\u20ac',
		'?': '\u00bf',
		'A': '\u00c5',
		'C': '\u00c7',
		'D': '\u00d0',
		'E': '\u00c9',
		'G': '\u011c',
		'H': '\u0124',
		'I': '\u00ce',
		'J': '\u0134',
		'K': '\u0136',
		'L': '\u013b',
		'N': '\u00d1',
		'O': '\u00d6',
		'P': '\u00de',
		'R': '\u00ae',
		'S': '\u0160',
		'T': '\u0162',
		'U': '\u00db',
		'W': '\u0174',
		'Y': '\u00dd',
		'Z': '\u017d',
		'a': '\u00e5',
		'c': '\u00e7',
		'd': '\u00f0',
		'e': '\u00e9',
		'f': '\u0192',
		'g': '\u011d',
		'h': '\u0125',
		'i': '\u00ee',
		'j': '\u0135',
		'k': '\u0137',
		'l': '\u013c',
		'n': '\u00f1',
		'o': '\u00f6',
		'p': '\u00fe',
		's': '\u0161',
		't': '\u0163',
		'u': '\u00fb',
		'w': '\u0175',
		'y': '\u00fd',
		'z': '\u017e',
	}
	result := []rune(english)
	for i, letter := range result {
		if accented, ok := accentedLetters[letter]; ok {
			result[i] = accented
		}
	}
	words := strings.Split(string(result), " ")
	if addNumbers {
		nWords := len(words)
		for i := 0; i < nWords; i++ {
			words = append(words, numberWords[i%len(numberWords)])
		}
	}
	return strings.Join(words, " ")
}

// nameAttribute creates a copy of the finder with the name attribute filled.
// If matchStart is specified, then it only matches a string starting with this text.
// If matchEnd is specified, then it only matches a string ending with this text.
// In RTL languages, matchStart and matchEnd will be reversed.
func (f *Finder) nameAttribute(english string, other map[string]string, matchStart, matchEnd bool) *Finder {
	c := f.copy()
	addName := func(lang, text string) {
		baseLanguage := strings.Split(lang, "-")[0]
		start := ""
		end := ""
		// It's unlikely we'll write tast tests for more than one RTL language, so just hardcode this.
		rtl := baseLanguage == "ar"
		if (matchEnd && rtl) || (matchStart && !rtl) {
			start = "^"
		}
		if (matchStart && rtl) || (matchEnd && !rtl) {
			end = "$"
		}
		c.name[lang] = *regexp.MustCompile(start + regexp.QuoteMeta(text) + end)
	}
	addName("en", english)
	// These are defaults, and can be overridden by using the other map
	addName("ar-XB", makeArXBString(english))
	// Only generate the number words when doing a precise match for the string.
	// Otherwise, you might search for a string "a b one two" in "a b c one two three".
	addName("en-XA", makeEnXAString(english, matchStart && matchEnd))
	for lang, text := range other {
		addName(lang, text)
	}
	return c
}

// Name creates a Finder with the specified name.
func Name(n string) *Finder {
	return newFinder().Name(n)
}

// Name creates a copy of the input Finder with the specified name.
func (f *Finder) Name(n string) *Finder {
	return f.MultilingualName(n, map[string]string{})
}

// MultilingualName creates a Finder with the specified names.
func MultilingualName(english string, other map[string]string) *Finder {
	return newFinder().MultilingualName(english, other)
}

// MultilingualName creates a copy of the input Finder with the specified names.
func (f *Finder) MultilingualName(english string, other map[string]string) *Finder {
	return f.nameAttribute(english, other, true, true)
}

// NameContaining creates a Finder with a name containing the specified string.
func NameContaining(n string) *Finder {
	return newFinder().NameContaining(n)
}

// NameContaining creates a copy of the input Finder with a name containing the specified string.
func (f *Finder) NameContaining(n string) *Finder {
	return f.MultilingualNameContaining(n, map[string]string{})
}

// MultilingualNameContaining creates a Finder with a name containing the specified strings.
func MultilingualNameContaining(english string, other map[string]string) *Finder {
	return newFinder().MultilingualNameContaining(english, other)
}

// MultilingualNameContaining creates a copy of the input Finder with a name containing the specified strings.
func (f *Finder) MultilingualNameContaining(english string, other map[string]string) *Finder {
	return f.nameAttribute(english, other, false, false)
}

// NameStartingWith creates a Finder with a name starting with the specified string.
func NameStartingWith(n string) *Finder {
	return newFinder().NameStartingWith(n)
}

// NameStartingWith creates a copy of the input Finder with a name starting with the specified string.
func (f *Finder) NameStartingWith(n string) *Finder {
	return f.MultilingualNameStartingWith(n, map[string]string{})
}

// MultilingualNameStartingWith creates a Finder with a name starting with the specified strings.
func MultilingualNameStartingWith(english string, other map[string]string) *Finder {
	return newFinder().MultilingualNameStartingWith(english, other)
}

// MultilingualNameStartingWith creates a copy of the input Finder with a name starting with the specified strings.
func (f *Finder) MultilingualNameStartingWith(english string, other map[string]string) *Finder {
	return f.nameAttribute(english, other, true, false)
}

// NameRegex creates a Finder with a name containing the specified regexp.
func NameRegex(r *regexp.Regexp) *Finder {
	return newFinder().NameRegex(r)
}

// MultilingualNameRegex creates a Finder with a name containing the specified regexp.
func MultilingualNameRegex(english *regexp.Regexp, other map[string]regexp.Regexp) *Finder {
	return newFinder().MultilingualNameRegex(english, other)
}

// NameRegex creates a copy of the input Finder with a name containing the specified regexp.
func (f *Finder) NameRegex(r *regexp.Regexp) *Finder {
	return f.MultilingualNameRegex(r, map[string]regexp.Regexp{})
}

// MultilingualNameRegex creates a copy of the input Finder with a name containing the specified regexp.
func (f *Finder) MultilingualNameRegex(english *regexp.Regexp, other map[string]regexp.Regexp) *Finder {
	c := f.copy()
	c.name = other
	c.name["en"] = *english
	return c
}

// ClassName creates a Finder with the specified class name.
// Deprecated: Use HasClass.
func ClassName(n string) *Finder {
	return Attribute("className", n)
}

// ClassName creates a copy of the input Finder with the specified class name.
// Deprecated: Use HasClass.
func (f *Finder) ClassName(n string) *Finder {
	return f.Attribute("className", n)
}

// HasClass creates a Finder with a class name containing the specified class name.
func HasClass(c string) *Finder {
	return newFinder().HasClass(c)
}

// HasClass creates a copy of the input Finder with a class name containing the specified class name.
func (f *Finder) HasClass(c string) *Finder {
	if _, ok := f.attributes["className"]; ok {
		panic("mutliple class names not supported")
	}
	return f.Attribute("className", regexp.MustCompile("\\b"+regexp.QuoteMeta(c)+"\\b"))
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

// Onscreen creates a Finder with Offscreen set to false.
func Onscreen() *Finder {
	return State(state.Offscreen, false)
}

// Onscreen creates a copy of the input Finder with Offscreen set to false.
func (f *Finder) Onscreen() *Finder {
	return f.State(state.Offscreen, false)
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
