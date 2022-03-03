package utils

import (
	"fmt"
	"strings"
)

type ActionState int

const (
	ActionUnplug ActionState = 0
	ActionPlugin ActionState = 1
	ActionFlip   ActionState = 2
)

// String returns a human-readable string representation for type ActionState.
func (a ActionState) String() string {
	switch a {
	case ActionUnplug:
		return "Unplug"
	case ActionPlugin:
		return "Plug in"
	case ActionFlip:
		return "Flip"
	default:
		return fmt.Sprintf("Unknown action state: %d", a)
	}
}

/*
Control switch fixture

Type:HDMI_Switch & TYPEC_Switch & TYPEA_Switch

Index:ID1 & ID2 & ID3....

cmd:

HDMI,0:Close All;1:PortA;2:PortB;3:PortC;4:PortD

Type-C,1:PortA;2:PortB AUS19129:(0:Close;1:Normal;2:Filp)

Type-A,1:PortA;2:PortB

DP,1:PortA;2PortB;3:Close

Ethernet,1:On;2:Off

resultCode：0000 成功

resultTxt：回應之訊息。

*/
func getHdmiPort(action ActionState) string {
	switch action {
	case ActionUnplug:
		return "0"
	case ActionPlugin:
		return "1"
	default:
		return ""
	}
}

func getTypeCPort(action ActionState) string {
	switch action {
	case ActionUnplug:
		return "2"
	case ActionPlugin:
		return "1"
	case ActionFlip:
		return "2"
	default:
		return ""
	}
}

func getTypeAPort(action ActionState) string {
	switch action {
	case ActionUnplug:
		return "2"
	case ActionPlugin:
		return "1"
	default:
		return ""
	}
}

func getEthernetPort(action ActionState) string {
	switch action {
	case ActionUnplug:
		return "2"
	case ActionPlugin:
		return "1"
	default:
		return ""
	}
}

func getDPPort(action ActionState) string {
	switch action {
	case ActionUnplug:
		return "2"
	case ActionPlugin:
		return "1"
	default:
		return ""
	}
}

func getPort(action ActionState, sType string) string {

	// hdmi
	if strings.Contains(sType, HDMI_Switch) {
		return getHdmiPort(action)
	}

	// type a
	if strings.Contains(sType, TYPEA_Switch) {
		return getTypeAPort(action)
	}

	// type c
	if strings.Contains(sType, TYPEC_Switch) {
		return getTypeCPort(action)
	}

	// dp
	if strings.Contains(sType, DP_Switch) {
		return getDPPort(action)
	}

	// ethernet
	if strings.Contains(sType, ETHERNET_Switch) {
		return getEthernetPort(action)
	}

	return ""

}
