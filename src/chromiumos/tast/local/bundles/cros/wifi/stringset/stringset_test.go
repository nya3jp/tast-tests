// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package stringset

import (
	"testing"
)

func TestStringSetString(t *testing.T) {
	for _, p := range []struct {
		in     []string
		expect string
	}{
		{nil, "StringSet([])"},
		{[]string{"a", "b", "c"}, "StringSet([\"a\" \"b\" \"c\"])"},
		{[]string{"b", "b", "c"}, "StringSet([\"b\" \"c\"])"},
		{[]string{"c", "b", "a"}, "StringSet([\"a\" \"b\" \"c\"])"},
	} {
		if actual := New(p.in).String(); actual != p.expect {
			t.Errorf("New(%q).String() = %q failed; expect: %q", p.in, actual, p.expect)
		}

	}
}

func TestStringSetUnion(t *testing.T) {
	for _, p := range []struct {
		a      StringSet
		b      StringSet
		expect StringSet
	}{
		{New(nil), New(nil), New(nil)},
		{New([]string{"a", "b", "c"}), New(nil), New([]string{"a", "b", "c"})},
		{New(nil), New([]string{"a", "b", "c"}), New([]string{"a", "b", "c"})},
		{New([]string{"a"}), New([]string{"a", "b", "c"}), New([]string{"a", "b", "c"})},
		{New([]string{"a", "b", "c"}), New([]string{"a"}), New([]string{"a", "b", "c"})},
		{New([]string{"a"}), New([]string{"b"}), New([]string{"a", "b"})},
		{New([]string{"a", "b"}), New([]string{"b", "c"}), New([]string{"a", "b", "c"})},
	} {
		if actual := p.a.Union(p.b); !p.expect.Equal(actual) {
			t.Errorf("%s.Union(%s) = %s failed; expect %s", p.a, p.b, actual, p.expect)
		}

	}

}

func TestStringSetDiff(t *testing.T) {
	for _, p := range []struct {
		a      StringSet
		b      StringSet
		expect StringSet
	}{
		{New(nil), New(nil), New(nil)},
		{New([]string{"a", "b", "c"}), New(nil), New([]string{"a", "b", "c"})},
		{New(nil), New([]string{"a", "b", "c"}), New(nil)},
		{New([]string{"a"}), New([]string{"a", "b", "c"}), New(nil)},
		{New([]string{"a", "b", "c"}), New([]string{"a"}), New([]string{"b", "c"})},
		{New([]string{"a"}), New([]string{"b"}), New([]string{"a"})},
		{New([]string{"a", "b"}), New([]string{"b", "c"}), New([]string{"a"})},
	} {
		if actual := p.a.Diff(p.b); !p.expect.Equal(actual) {
			t.Errorf("%s.Diff(%s) = %s failed; expect %s", p.a, p.b, actual, p.expect)
		}

	}
}

func TestStringSetIntersect(t *testing.T) {
	for _, p := range []struct {
		a      StringSet
		b      StringSet
		expect StringSet
	}{
		{New(nil), New(nil), New(nil)},
		{New([]string{"a", "b", "c"}), New(nil), New(nil)},
		{New(nil), New([]string{"a", "b", "c"}), New(nil)},
		{New([]string{"a"}), New([]string{"a", "b", "c"}), New([]string{"a"})},
		{New([]string{"a", "b", "c"}), New([]string{"a"}), New([]string{"a"})},
		{New([]string{"a"}), New([]string{"b"}), New(nil)},
		{New([]string{"a", "b"}), New([]string{"b", "c"}), New([]string{"b"})},
	} {
		if actual := p.a.Intersect(p.b); !p.expect.Equal(actual) {
			t.Errorf("%s.Intersect(%s) = %s failed; expect %s", p.a, p.b, actual, p.expect)
		}

	}
}

func TestStringSetHas(t *testing.T) {
	for _, p := range []struct {
		a      StringSet
		s      string
		expect bool
	}{
		{New(nil), "a", false},
		{New([]string{"a", "b", "c"}), "a", true},
		{New([]string{"a", "b", "c"}), "d", false},
	} {
		if actual := p.a.Has(p.s); actual != p.expect {
			t.Errorf("%s.Has(%s) = %t failed; expect %t", p.a, p.s, actual, p.expect)
		}
	}
}

func TestStringSetEqual(t *testing.T) {
	for _, p := range []struct {
		a      StringSet
		b      StringSet
		expect bool
	}{
		{New(nil), New(nil), true},
		{New([]string{"a", "b", "c"}), New(nil), false},
		{New(nil), New([]string{"a", "b", "c"}), false},
		{New([]string{"c", "b", "a"}), New([]string{"a", "b", "c"}), true},
		{New([]string{"a", "b", "c"}), New([]string{"a", "b"}), false},
		{New([]string{"a", "b"}), New([]string{"a", "b", "c"}), false},
	} {
		if actual := p.a.Equal(p.b); actual != p.expect {
			t.Errorf("%s.Equal(%s) = %t failed; expect %t", p.a, p.b, actual, p.expect)
		}

	}
}
