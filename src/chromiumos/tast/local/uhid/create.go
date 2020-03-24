package uhid

const UHIDCreate2 uint32 = 11

// struct attempts to replicate C struct found here:
// https://cs.corp.google.com/chromeos_public/src/third_party/kernel/v4.14/include/uapi/linux/uhid.h?pv=1&l=45
// A create request is used to create a device that responds to the
// given data.
type UHIDCreate2Request struct {
	RequestType    uint32
	Name           [128]byte
	Phys           [64]byte
	Uniq           [64]byte
	DescriptorSize uint16
	Bus            uint16
	VendorId       uint32
	ProductId      uint32
	Version        uint32
	Country        uint32
	Descriptor     [HIDMaxDescriptorSize]byte
}

// createRequest is used as a constructor for a UHIDCreate2Request
// based on the data contained in DeviceData.
func createRequest(d DeviceData) UHIDCreate2Request {
	r := UHIDCreate2Request{}
	r.RequestType = UHIDCreate2
	r.Name = d.Name
	r.Phys = d.Phys
	r.Uniq = d.Uniq
	r.DescriptorSize = uint16(len(d.Descriptor))
	r.Bus = d.Bus
	r.VendorId = d.VendorId
	r.ProductId = d.ProductId
	r.Version = 0
	r.Country = 0
	r.Descriptor = d.Descriptor
	return r
}
