package utils

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
	"context"
	"fmt"
	"strconv"
	"strings"
)

// fixture web api unit url
const (
	FixtureWebUrl = "FixtureWebUrl"
)

//describe peripheral type
const (
	PeripheralUsb = "PeripheralUsb"
)

// define object to parse input vars
type InputArgument struct {
	PeripheralType    string
	SwitchFixtureType string
	StartIndex        int
	Count             int
}

// return input vars
func GetInputVars() []string {
	want := GetInputTypeList()
	want = append(want, FixtureWebUrl)
	return want
}

// return peripherals type
func GetPeriperalList() []string {
	return []string{
		PeripheralUsb,
	}
}

// input var type as follow:
// -var=PeripheralType.SwitchFixtureType=StartPosition,Count
func GetInputTypeList() []string {
	var want []string
	for _, perp := range GetPeriperalList() {
		for _, sw := range GetSwitchList() {
			want = append(want, fmt.Sprintf("%s.%s", perp, sw))
		}
	}

	return want
}

// parse input vars into specific format
// return list of InputArgument
func ParseInputVars(ctx context.Context, s *testing.State) ([]InputArgument, error) {
	var args []InputArgument

	// parse input perp
	for _, perpType := range GetInputTypeList() {

		if variable, ok := s.Var(perpType); ok && variable != "" {

			arg := new(InputArgument)

			// deal with perpherals & switch fixture
			arr := strings.Split(perpType, ".")
			if len(arr) != 2 {
				return nil, errors.Errorf("Failed to split to two types: ")
			}
			arg.PeripheralType = arr[0]
			arg.SwitchFixtureType = arr[1]

			// deal with startIndex
			arr = strings.Split(variable, ",")
			startIndex, err := strconv.Atoi(arr[0])
			if err != nil {
				return nil, errors.Wrap(err, "Failed to convert startIndex to int: ")
			} else {
				arg.StartIndex = startIndex
			}

			// deal with count
			count, err := strconv.Atoi(arr[1])
			if err != nil {
				return nil, errors.Wrap(err, "Failed to convert count to int: ")
			} else {
				arg.Count = count
			}

			args = append(args, *arg)
		}
	}

	return args, nil
}

// according to want perpheral tpye,
// return list of InputArgument
func GetInputArgument(ctx context.Context, s *testing.State, wantPerpType string) ([]InputArgument, error) {

	var want []InputArgument

	args, err := ParseInputVars(ctx, s)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse input variabels: ")
	}

	for _, arg := range args {
		if arg.PeripheralType == wantPerpType {
			want = append(want, arg)
		}
	}

	return want, nil
}

// according to want peripheral type,
// return total count of specific peripheral of inputArgument
func GetInputArgumentCount(ctx context.Context, s *testing.State, wantPerpType string) (int, error) {

	args, err := GetInputArgument(ctx, s, wantPerpType)
	if err != nil {
		return -1, errors.Wrapf(err, "Failed to get input arguments - %s: ", wantPerpType)
	}

	var count int
	count = 0
	for _, arg := range args {
		count += arg.Count
	}

	return count, nil
}
