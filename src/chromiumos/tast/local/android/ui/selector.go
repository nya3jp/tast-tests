// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"fmt"
	"strings"
)

// Definitions in this file must match with:
// https://github.com/xiaocong/android-uiautomator-server/blob/master/app/src/androidTest/java/com/github/uiautomator/stub/Selector.java

// selector holds UI element selection criteria.
//
// This object corresponds to UiSelector in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector
type selector struct {
	Text                  string `json:"text,omitempty"`
	TextContains          string `json:"textContains,omitempty"`
	TextMatches           string `json:"textMatches,omitempty"`
	TextStartsWith        string `json:"textStartsWith,omitempty"`
	ClassName             string `json:"className,omitempty"`
	ClassNameMatches      string `json:"classNameMatches,omitempty"`
	Description           string `json:"description,omitempty"`
	DescriptionContains   string `json:"descriptionContains,omitempty"`
	DescriptionMatches    string `json:"descriptionMatches,omitempty"`
	DescriptionStartsWith string `json:"descriptionStartsWith,omitempty"`
	Checkable             bool   `json:"checkable,omitempty"`
	Checked               bool   `json:"checked,omitempty"`
	Clickable             bool   `json:"clickable,omitempty"`
	LongClickable         bool   `json:"longClickable,omitempty"`
	Scrollable            bool   `json:"scrollable,omitempty"`
	Enabled               bool   `json:"enabled,omitempty"`
	Focusable             bool   `json:"focusable,omitempty"`
	Focused               bool   `json:"focused,omitempty"`
	Selected              bool   `json:"selected,omitempty"`
	PackageName           string `json:"packageName,omitempty"`
	PackageNameMatches    string `json:"packageNameMatches,omitempty"`
	ResourceID            string `json:"resourceId,omitempty"`
	ResourceIDMatches     string `json:"resourceIdMatches,omitempty"`
	Index                 int    `json:"index,omitempty"`
	Instance              int    `json:"instance,omitempty"`

	Mask uint32 `json:"mask"`
}

const (
	maskText                  = 0x01
	maskTextContains          = 0x02
	maskTextMatches           = 0x04
	maskTextStartsWith        = 0x08
	maskClassName             = 0x10
	maskClassNameMatches      = 0x20
	maskDescription           = 0x40
	maskDescriptionContains   = 0x80
	maskDescriptionMatches    = 0x0100
	maskDescriptionStartsWith = 0x0200
	maskCheckable             = 0x0400
	maskChecked               = 0x0800
	maskClickable             = 0x1000
	maskLongClickable         = 0x2000
	maskScrollable            = 0x4000
	maskEnabled               = 0x8000
	maskFocusable             = 0x010000
	maskFocused               = 0x020000
	maskSelected              = 0x040000
	maskPackageName           = 0x080000
	maskPackageNameMatches    = 0x100000
	maskResourceID            = 0x200000
	maskResourceIDMatches     = 0x400000
	maskIndex                 = 0x800000
	maskInstance              = 0x01000000
)

// SelectorOption specifies UI element selection criteria.
type SelectorOption func(s *selector)

// newSelector is called from Object to construct selector.
func newSelector(opts []SelectorOption) *selector {
	s := &selector{}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Text limits the selection criteria by text property.
//
// This corresponds to UiSelector.text in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#text(java.lang.String)
func Text(text string) SelectorOption {
	return func(s *selector) {
		s.Text = text
		s.Mask |= maskText
	}
}

// TextContains limits the selection criteria by substring of text property.
//
// This corresponds to UiSelector.textContains in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#textContains(java.lang.String)
func TextContains(text string) SelectorOption {
	return func(s *selector) {
		s.TextContains = text
		s.Mask |= maskTextContains
	}
}

// TextMatches limits the selection criteria by regex matching text property.
//
// This corresponds to UiSelector.textMatches in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#textMatches(java.lang.String)
func TextMatches(regex string) SelectorOption {
	return func(s *selector) {
		s.TextMatches = regex
		s.Mask |= maskTextMatches
	}
}

// TextStartsWith limits the selection criteria by prefix of text property.
//
// This corresponds to UiSelector.textStartsWith in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#textStartsWith(java.lang.String)
func TextStartsWith(text string) SelectorOption {
	return func(s *selector) {
		s.TextStartsWith = text
		s.Mask |= maskTextStartsWith
	}
}

// ClassName limits the selection criteria by class property.
//
// This corresponds to UiSelector.className in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#className(java.lang.String)
func ClassName(name string) SelectorOption {
	return func(s *selector) {
		s.ClassName = name
		s.Mask |= maskClassName
	}
}

// ClassNameMatches limits the selection criteria by regex matching class property.
//
// This corresponds to UiSelector.classNameMatches in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#classNameMatches(java.lang.String)
func ClassNameMatches(name string) SelectorOption {
	return func(s *selector) {
		s.ClassNameMatches = name
		s.Mask |= maskClassNameMatches
	}
}

// Description limits the selection criteria by description property.
//
// This corresponds to UiSelector.description in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#description(java.lang.String)
func Description(desc string) SelectorOption {
	return func(s *selector) {
		s.Description = desc
		s.Mask |= maskDescription
	}
}

// DescriptionContains limits the selection criteria by substring of description property.
//
// This corresponds to UiSelector.descriptionContains in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#descriptionContains(java.lang.String)
func DescriptionContains(desc string) SelectorOption {
	return func(s *selector) {
		s.DescriptionContains = desc
		s.Mask |= maskDescriptionContains
	}
}

// DescriptionMatches limits the selection criteria by regex matching description property.
//
// This corresponds to UiSelector.descriptionMatches in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#descriptionMatches(java.lang.String)
func DescriptionMatches(regex string) SelectorOption {
	return func(s *selector) {
		s.DescriptionMatches = regex
		s.Mask |= maskDescriptionMatches
	}
}

// DescriptionStartsWith limits the selection criteria by prefix of description property.
//
// This corresponds to UiSelector.descriptionStartsWith in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#descriptionStartsWith(java.lang.String)
func DescriptionStartsWith(desc string) SelectorOption {
	return func(s *selector) {
		s.DescriptionStartsWith = desc
		s.Mask |= maskDescriptionStartsWith
	}
}

// Checkable limits the selection criteria by if an object is checkable.
//
// This corresponds to UiSelector.checkable in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#checkable(boolean)
func Checkable(b bool) SelectorOption {
	return func(s *selector) {
		s.Checkable = b
		s.Mask |= maskCheckable
	}
}

// Checked limits the selection criteria by if an object is checked.
//
// This corresponds to UiSelector.checked in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#checked(boolean)
func Checked(b bool) SelectorOption {
	return func(s *selector) {
		s.Checked = b
		s.Mask |= maskChecked
	}
}

// Clickable limits the selection criteria by if an object is clickable.
//
// This corresponds to UiSelector.clickable in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#clickable(boolean)
func Clickable(b bool) SelectorOption {
	return func(s *selector) {
		s.Clickable = b
		s.Mask |= maskClickable
	}
}

// LongClickable limits the selection criteria by if an object is long-clickable.
//
// This corresponds to UiSelector.longClickable in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#longClickable(boolean)
func LongClickable(b bool) SelectorOption {
	return func(s *selector) {
		s.LongClickable = b
		s.Mask |= maskLongClickable
	}
}

// Scrollable limits the selection criteria by if an object is scrollable.
//
// This corresponds to UiSelector.scrollable in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#scrollable(boolean)
func Scrollable(b bool) SelectorOption {
	return func(s *selector) {
		s.Scrollable = b
		s.Mask |= maskScrollable
	}
}

// Enabled limits the selection criteria by if an object is enabled.
//
// This corresponds to UiSelector.enabled in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#enabled(boolean)
func Enabled(b bool) SelectorOption {
	return func(s *selector) {
		s.Enabled = b
		s.Mask |= maskEnabled
	}
}

// Focusable limits the selection criteria by if an object is focusable.
//
// This corresponds to UiSelector.focusable in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#focusable(boolean)
func Focusable(b bool) SelectorOption {
	return func(s *selector) {
		s.Focusable = b
		s.Mask |= maskFocusable
	}
}

// Focused limits the selection criteria by if an object is focused.
//
// This corresponds to UiSelector.focused in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#focused(boolean)
func Focused(b bool) SelectorOption {
	return func(s *selector) {
		s.Focused = b
		s.Mask |= maskFocused
	}
}

// Selected limits the selection criteria by if an object is selected.
//
// This corresponds to UiSelector.selected in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#selected(boolean)
func Selected(b bool) SelectorOption {
	return func(s *selector) {
		s.Selected = b
		s.Mask |= maskSelected
	}
}

// PackageName limits the selection criteria by package name.
//
// This corresponds to UiSelector.packageName in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#packageName(java.lang.String)
func PackageName(pkg string) SelectorOption {
	return func(s *selector) {
		s.PackageName = pkg
		s.Mask |= maskPackageName
	}
}

// PackageNameMatches limits the selection criteria by regex matching package name.
//
// This corresponds to UiSelector.packageNameMatches in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#packageNameMatches(java.lang.String)
func PackageNameMatches(regex string) SelectorOption {
	return func(s *selector) {
		s.PackageNameMatches = regex
		s.Mask |= maskPackageNameMatches
	}
}

// ResourceID limits the selection criteria by resource ID.
//
// This corresponds to UiSelector.resourceId in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#resourceId(java.lang.String)
func ResourceID(resourceID string) SelectorOption {
	return func(s *selector) {
		s.ResourceID = resourceID
		s.Mask |= maskResourceID
	}
}

// ResourceIDMatches limits the selection criteria by regex matching resource ID.
//
// This corresponds to UiSelector.resourceIdMatches in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#resourceIdMatches(java.lang.String)
func ResourceIDMatches(regex string) SelectorOption {
	return func(s *selector) {
		s.ResourceIDMatches = regex
		s.Mask |= maskResourceIDMatches
	}
}

// Index limits the selection criteria by node index.
//
// This corresponds to UiSelector.index in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#index(int)
func Index(i int) SelectorOption {
	return func(s *selector) {
		s.Index = i
		s.Mask |= maskIndex
	}
}

// Instance limits the selection criteria by instance number.
//
// This corresponds to UiSelector.instance in UI Automator API:
// https://developer.android.com/reference/android/support/test/uiautomator/UiSelector.html#instance(int)
func Instance(i int) SelectorOption {
	return func(s *selector) {
		s.Instance = i
		s.Mask |= maskInstance
	}
}

// ID is an alias of ResourceID.
func ID(resourceID string) SelectorOption {
	return ResourceID(resourceID)
}

// String implements fmt.Stringer interface.
func (s *selector) String() string {
	var v []string
	if s.Mask&maskText != 0 {
		v = append(v, fmt.Sprintf("Text:%+v", s.Text))
	}
	if s.Mask&maskTextContains != 0 {
		v = append(v, fmt.Sprintf("TextContains:%+v", s.TextContains))
	}
	if s.Mask&maskTextMatches != 0 {
		v = append(v, fmt.Sprintf("TextMatches:%+v", s.TextMatches))
	}
	if s.Mask&maskTextStartsWith != 0 {
		v = append(v, fmt.Sprintf("TextStartsWith:%+v", s.TextStartsWith))
	}
	if s.Mask&maskClassName != 0 {
		v = append(v, fmt.Sprintf("ClassName:%+v", s.ClassName))
	}
	if s.Mask&maskClassNameMatches != 0 {
		v = append(v, fmt.Sprintf("ClassNameMatches:%+v", s.ClassNameMatches))
	}
	if s.Mask&maskDescription != 0 {
		v = append(v, fmt.Sprintf("Description:%+v", s.Description))
	}
	if s.Mask&maskDescriptionContains != 0 {
		v = append(v, fmt.Sprintf("DescriptionContains:%+v", s.DescriptionContains))
	}
	if s.Mask&maskDescriptionMatches != 0 {
		v = append(v, fmt.Sprintf("DescriptionMatches:%+v", s.DescriptionMatches))
	}
	if s.Mask&maskDescriptionStartsWith != 0 {
		v = append(v, fmt.Sprintf("DescriptionStartsWith:%+v", s.DescriptionStartsWith))
	}
	if s.Mask&maskCheckable != 0 {
		v = append(v, fmt.Sprintf("Checkable:%+v", s.Checkable))
	}
	if s.Mask&maskChecked != 0 {
		v = append(v, fmt.Sprintf("Checked:%+v", s.Checked))
	}
	if s.Mask&maskClickable != 0 {
		v = append(v, fmt.Sprintf("Clickable:%+v", s.Clickable))
	}
	if s.Mask&maskLongClickable != 0 {
		v = append(v, fmt.Sprintf("LongClickable:%+v", s.LongClickable))
	}
	if s.Mask&maskScrollable != 0 {
		v = append(v, fmt.Sprintf("Scrollable:%+v", s.Scrollable))
	}
	if s.Mask&maskEnabled != 0 {
		v = append(v, fmt.Sprintf("Enabled:%+v", s.Enabled))
	}
	if s.Mask&maskFocusable != 0 {
		v = append(v, fmt.Sprintf("Focusable:%+v", s.Focusable))
	}
	if s.Mask&maskFocused != 0 {
		v = append(v, fmt.Sprintf("Focused:%+v", s.Focused))
	}
	if s.Mask&maskSelected != 0 {
		v = append(v, fmt.Sprintf("Selected:%+v", s.Selected))
	}
	if s.Mask&maskPackageName != 0 {
		v = append(v, fmt.Sprintf("PackageName:%+v", s.PackageName))
	}
	if s.Mask&maskPackageNameMatches != 0 {
		v = append(v, fmt.Sprintf("PackageNameMatches:%+v", s.PackageNameMatches))
	}
	if s.Mask&maskResourceID != 0 {
		v = append(v, fmt.Sprintf("ResourceId:%+v", s.ResourceID))
	}
	if s.Mask&maskResourceIDMatches != 0 {
		v = append(v, fmt.Sprintf("ResourceIdMatches:%+v", s.ResourceIDMatches))
	}
	if s.Mask&maskIndex != 0 {
		v = append(v, fmt.Sprintf("Index:%+v", s.Index))
	}
	if s.Mask&maskInstance != 0 {
		v = append(v, fmt.Sprintf("Instance:%+v", s.Instance))
	}
	return fmt.Sprintf("{%s}", strings.Join(v, " "))
}
