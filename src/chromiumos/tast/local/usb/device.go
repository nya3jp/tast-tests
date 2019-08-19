package usb

// Binary Coded Decimal
type BCD uint16

// DeviceDescriptor holds basic info about USB device.
type DeviceInfo struct {
	UsbRev             BCD
	DeviceClass        uint8
	DeviceSubClass     uint8
	DeviceProtocol     uint8
	VendorId           uint16
	ProductId          uint16
	DeviceRev          BCD
	Manufacturer       string
	Product            string
	SerialNumber       string
}

