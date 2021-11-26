// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package useractions

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/errors"
)

func TestLogCleanActionResult(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(3 * time.Second)

	result := &actionResult{
		actionName: "actionName",
		testName:   "testName",
		attributes: map[string]string{},
		tags:       []ActionTag{},
		startTime:  startTime,
		endTime:    endTime,
		pass:       true,
		err:        nil,
	}

	got, err := result.stringArray()
	if err != nil {
		t.Fatal("Failed to parse action result: ", err)
	}

	want := []string{"actionName", "testName", "{}", "", startTime.Format(actionTimeFormat), endTime.Format(actionTimeFormat), "true", ""}

	if !cmp.Equal(got, want) {
		t.Errorf("failed to format action result; got %+v, want %+v", got, want)
	}
}

func TestLogActionResultWithAttributesAndTags(t *testing.T) {
	const (
		ActionTagTest1 ActionTag = "TestTag1"
		ActionTagTest2 ActionTag = "TestTag2"
	)

	startTime := time.Now()
	endTime := startTime.Add(3 * time.Second)

	result := &actionResult{
		actionName: "actionName",
		testName:   "testName",
		attributes: map[string]string{"TestAttributeName": "TestAttributeValue"},
		tags:       []ActionTag{ActionTagTest1, ActionTagTest2},
		startTime:  startTime,
		endTime:    endTime,
		pass:       false,
		err:        errors.New("Test Error"),
	}

	got, err := result.stringArray()
	if err != nil {
		t.Fatal("Failed to parse action result: ", err)
	}

	want := []string{"actionName", "testName", "{\"TestAttributeName\":\"TestAttributeValue\"}", "TestTag1, TestTag2", startTime.Format(actionTimeFormat), endTime.Format(actionTimeFormat), "false", "Test Error"}

	if !cmp.Equal(got, want) {
		t.Errorf("failed to format action result; got %+v, want %+v", got, want)
	}
}
