// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// fixture web api unit url
const (
	FixtureWebURL = "FixtureWebUrl"
)

//describe peripheral type
const (
	PeripheralUsb = "PeripheralUsb"
)

// InputArgument define object to parse input vars
type InputArgument struct {
	PeripheralType    string
	SwitchFixtureType string
	StartIndex        int
	Count             int
}

// GetInputVars return input vars
func GetInputVars() []string {
	want := GetInputTypeList()
	want = append(want, FixtureWebURL)
	return want
}

// GetPeriperalList return peripherals type
func GetPeriperalList() []string {
	return []string{
		PeripheralUsb,
	}
}

// GetInputTypeList -var=PeripheralType.SwitchFixtureType=StartPosition,Count
func GetInputTypeList() []string {
	var want []string
	for _, perp := range GetPeriperalList() {
		for _, sw := range GetSwitchList() {
			want = append(want, fmt.Sprintf("%s.%s", perp, sw))
		}
	}

	return want
}

// ParseInputVars parse input vars into specific format return list of InputArgument
func ParseInputVars(ctx context.Context, s *testing.State) ([]InputArgument, error) {
	var args []InputArgument

	// parse input perp
	for _, perpType := range GetInputTypeList() {

		if variable, ok := s.Var(perpType); ok && variable != "" {

			arg := new(InputArgument)

			// deal with perpherals & switch fixture
			arr := strings.Split(perpType, ".")
			if len(arr) != 2 {
				return nil, errors.New("failed to split to two types: ")
			}
			arg.PeripheralType = arr[0]
			arg.SwitchFixtureType = arr[1]

			// deal with startIndex
			arr = strings.Split(variable, ",")
			startIndex, err := strconv.Atoi(arr[0])
			if err != nil {
				return nil, errors.Wrap(err, "failed to convert startIndex to int")
			}
			arg.StartIndex = startIndex

			// deal with count
			count, err := strconv.Atoi(arr[1])
			if err != nil {
				return nil, errors.Wrap(err, "failed to convert count to int")
			}
			arg.Count = count

			args = append(args, *arg)
		}
	}

	return args, nil
}

// GetInputArgument  according to want perpheral tpye, return list of InputArgument
func GetInputArgument(ctx context.Context, s *testing.State, wantPerpType string) ([]InputArgument, error) {

	var want []InputArgument

	args, err := ParseInputVars(ctx, s)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse input variabels")
	}

	for _, arg := range args {
		if arg.PeripheralType == wantPerpType {
			want = append(want, arg)
		}
	}

	return want, nil
}

// GetInputArgumentCount according to want peripheral type, return total count of specific peripheral of inputArgument
func GetInputArgumentCount(ctx context.Context, s *testing.State, wantPerpType string) (int, error) {

	args, err := GetInputArgument(ctx, s, wantPerpType)
	if err != nil {
		return -1, errors.Wrapf(err, "failed to get input arguments - %s: ", wantPerpType)
	}

	var count int
	count = 0
	for _, arg := range args {
		count += arg.Count
	}

	return count, nil
}
