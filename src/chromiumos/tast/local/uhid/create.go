package uhid

const uhidCreate2 uint32 = 11

// uhidCreate2Request replicates C struct found here:
// https://cs.corp.google.com/chromeos_public/src/third_party/kernel/v4.14/include/uapi/linux/uhid.h?pv=1&l=45
// Create requests are written into /dev/uhid in order to create a
// virtual hid device. This device will have the given name and IDs as
// well as respond to the given HID descriptor.
type uhidCreate2Request struct {
	RequestType    uint32
	Name           [128]byte
	Phys           [64]byte
	Uniq           [64]byte
	DescriptorSize uint16
	Bus            uint16
	VendorID       uint32
	ProductID      uint32
	Version        uint32
	Country        uint32
	Descriptor     [hidMaxDescriptorSize]byte
}

// createRequest returns a new uhidCreate2Request based on the data
// contained in deviceData.
func createRequest(d DeviceData) uhidCreate2Request {
	r := uhidCreate2Request{}
	r.RequestType = uhidCreate2
	r.Name = d.Name
	r.Phys = d.Phys
	r.Uniq = d.Uniq
	r.DescriptorSize = uint16(len(d.Descriptor))
	r.Bus = d.Bus
	r.VendorID = d.VendorID
	r.ProductID = d.ProductID
	r.Version = 0
	r.Country = 0
	r.Descriptor = d.Descriptor
	return r
}
