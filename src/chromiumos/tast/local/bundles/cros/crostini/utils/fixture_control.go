package utils

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

type FixtureType int

const (
	FixtureStation  FixtureType = 0
	FixtureExtDisp1 FixtureType = 1
	FixtureExtDisp2 FixtureType = 2
	FixtureEthernet FixtureType = 3
)

type SwitchType string
type SwitchIndex string

type SwitchParams struct {
	SwitchType  SwitchType
	SwitchIndex SwitchIndex
	Action      ActionState
	NeedToDelay bool
}

// String returns a human-readable string representation for type FixtureType.
func (f FixtureType) String() string {
	switch f {
	case FixtureStation:
		return "Station"
	case FixtureExtDisp1:
		return "External display 1"
	case FixtureExtDisp2:
		return "External display 2"
	case FixtureEthernet:
		return "Ethernet"
	default:
		return fmt.Sprintf("Unknown fixture type: %d", f)
	}
}

// according fixtureType
// return switch fixture type & index
func getFixtureTypeAndIndex(fixture FixtureType) (string, string) {
	switch fixture {
	case FixtureStation:
		return StationType, StationIndex
	case FixtureExtDisp1:
		return ExtDisp1Type, ExtDisp1Index
	case FixtureExtDisp2:
		return ExtDisp2Type, ExtDisp2Index
	case FixtureEthernet:
		return EthernetType, EthernetIndex
	default:
		return "", ""
	}
}

// get time for delaying switch fixture
func getInterval(needToDelay bool) string {
	if needToDelay {
		return "5"
	} else {
		return "0"
	}
}

//  waiting for system response
func waitForChromebook(needToDelay bool) {

	var delayTime int
	if needToDelay {
		delayTime = 1
	} else {
		delayTime = 10
	}

	time.Sleep(time.Duration(delayTime) * time.Second)
}

func DoSwitchFixture(ctx context.Context, s *testing.State, sType, sIndex string, action ActionState, needToDelay bool) error {

	interval := getInterval(needToDelay)

	// restrict input range
	if action < ActionUnplug || action > ActionFlip {
		return errors.Errorf("Incorrect action value: got %d, want [%d - %d]", action, ActionUnplug, ActionFlip)
	}

	// according to input action & fixture
	// to correspond port to switch
	port := getPort(action, sType)
	if port == "" {
		return errors.New("failed to get correspond port: ")
	}

	// according parameter, to switch fixture
	if err := SwitchFixture(s, sType, sIndex, port, interval); err != nil {
		return errors.Wrap(err, "failed to execute SwitchFixture")
	}

	// waiting for chromebook response
	waitForChromebook(needToDelay)

	return nil
}

// this method may need to update for input argument
// like: get input argument
// 		then control fixture by argument
func ControlFixture(ctx context.Context, s *testing.State, fixture FixtureType, action ActionState, needToDelay bool) error {

	s.Logf("%s - %s ", action.String(), fixture.String())

	// according to input fixture
	// return correspond type & index
	fType, fIndex := getFixtureTypeAndIndex(fixture)

	if err := DoSwitchFixture(ctx, s, fType, fIndex, action, needToDelay); err != nil {
		return err
	}

	return nil
}

// control all peripherals
// such as ext-display1, ethernet, usbs
func ControlPeripherals(ctx context.Context, s *testing.State, uc *UsbController, action ActionState, needToDelay bool) error {

	// ext-display 1
	if err := ControlFixture(ctx, s, FixtureExtDisp1, action, needToDelay); err != nil {
		return err
	}

	// ethernet
	if err := ControlFixture(ctx, s, FixtureEthernet, action, needToDelay); err != nil {
		return err
	}

	// usbs
	if err := uc.ControlUsbs(ctx, s, action, needToDelay); err != nil {
		return err
	}

	// audio
	return nil
}

// according to input argument
// switch fixture one by one
func ControlFixtureByArgument(ctx context.Context, s *testing.State, args []InputArgument, action ActionState, needToDelay bool) error {

	for _, arg := range args {
		for i := 0; i < arg.Count; i++ {
			sIndex := fmt.Sprintf("ID%d", arg.StartIndex+i)
			if err := DoSwitchFixture(ctx, s, arg.SwitchFixtureType, sIndex, action, needToDelay); err != nil {
				return err
			}

		}
	}

	return nil
}
