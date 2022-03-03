package utils

//-------------------------------------------------------------------------------

// self-define

// list switch types
const (
	HDMI_Switch     = "HDMI_Switch"
	TYPEA_Switch    = "TYPEA_Switch"
	TYPEC_Switch    = "TYPEC_Switch"
	VGA_Switch      = "VGA_Switch"
	DP_Switch       = "DP_Switch"
	DVI_Switch      = "DVI_Switch"
	ETHERNET_Switch = "ETHERNET_Switch"
)

func GetSwitchList() []string {
	return []string{
		HDMI_Switch,
		TYPEA_Switch,
		TYPEC_Switch,
		VGA_Switch,
		DP_Switch,
		DVI_Switch,
		ETHERNET_Switch,
	}
}

// -----------------------------------------------------------------------------

// fixture type & id settings

// station
const (
	StationType  = "Docking_TYPEC_Switch"
	StationIndex = "ID1"
)

// display
const (

	// build in display
	// type is blank
	IntDispType  = ""
	IntDispIndex = "ID1"

	// first external display using hdmi
	ExtDisp1Type  = "Display_HDMI_Switch"
	ExtDisp1Index = "ID2"

	// second external display using dp
	ExtDisp2Type  = "Display_DP_Switch"
	ExtDisp2Index = "ID2"
)

// ethernet
const (
	EthernetType  = "ETHERNET_Switch"
	EthernetIndex = "ID1"
)

// usb print
const (
	UsbPrinterType  = "TYPEA_Switch"
	UsbPrinterIndex = "ID1"
)

// camera
const (
	CameraType  = "TYPEA_Switch"
	CameraIndex = "ID1"
)

// microphone
const (
	MicrophoneType  = "TYPEA_Switch"
	MicrophoneIndex = "ID1"
)

//  -------------------------------
