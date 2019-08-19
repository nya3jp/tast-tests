package usb

// USB Port
type DevicePort interface {
	Name() string
}

// Simple loopback UDC
type loopbackDevicePort struct {}

func LoopbackDevicePort() DevicePort {
	return &loopbackDevicePort{}
}

func (_ *loopbackDevicePort) Name() string {
	return "dummy_udc.0"
}
