// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package useractions

import (
	"reflect"
	"testing"
	"time"

	"chromiumos/tast/errors"
)

func TestLogCleanActionResult(t *testing.T) {
	result := &actionResult{
		actionName: "actionName",
		testName:   "testName",
		attributes: map[string]string{},
		tags:       []string{},
		duration:   1 * time.Second,
		pass:       true,
		err:        nil,
	}

	got, err := result.stringArray()
	if err != nil {
		t.Fatal("Failed to parse action result: ", err)
	}

	want := []string{"actionName", "testName", "{}", "", "1000", "true", ""}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("failed to format action result; want %+v, got %+v", want, got)
	}
}

func TestLogActionResultWithAttributesAndTags(t *testing.T) {
	result := &actionResult{
		actionName: "actionName",
		testName:   "testName",
		attributes: map[string]string{"TestAttributeName": "TestAttributeValue"},
		tags:       []string{"TestTag1", "TestTag2"},
		duration:   1 * time.Second,
		pass:       false,
		err:        errors.New("Test Error"),
	}

	got, err := result.stringArray()
	if err != nil {
		t.Fatal("Failed to parse action result: ", err)
	}

	want := []string{"actionName", "testName", "{\"TestAttributeName\":\"TestAttributeValue\"}", "TestTag1, TestTag2", "1000", "false", "Test Error"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("failed to format action result; want %+v, got %+v", want, got)
	}
}
