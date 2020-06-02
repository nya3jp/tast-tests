# Gamepad Tests

These tests attempt to validate the proper functioning of gamepads on ChromeOS.
So far the tests only validate that the mappings for the buttons on the
controllers are correct.

## Creating a test

### HID recording

The recordings used to create virtual devices and replay events are obtained
from running the hid-recorder command found in
https://gitlab.freedesktop.org/libevdev/hid-tools. An example usage of the
command would be:

`hid-recorder /dev/hidraw0 --output recordings/ds4.hid`

where /dev/hidraw0 is the hidraw node of a dualshock 4 controller. If one
doesn't know the hidraw node of the controller the command can be run without a
path and the command will list available hidraw nodes with device names for the
user to select.

When creating the recording it is helpful to plan in advance in what order the
buttons will be pressed since it will be necessary to state that in the test.

An example of an expected button array that could be used in a test is:

```
expectedButtons := []string{
  "triangle",
  "circle",
  "x",
  "square",
}
```

During the recording one should only press each button once since the test
ignores repeats. For example, if one were to press buttons in the sequence.
triangle, triangle, square, x, square, circle. During the test that would simply
be read as triangle, square, x, circle. This is to prevent repeats that arise
from reading two consecutive gamepad states that occur after a button is pressed
but before it is unpressed.

### Figuring out the javascript button mappings

The test also requires that you supply the button mappings between the
javascript gamepad API and the actual gamepad buttons. The buttons in this API
consist of an array in which each index has a button object. Thus, an example of
a possible button mapping is:

```
  buttonMappings := `{
    0: "x",
    1: "circle",
    2: "square",
    3: "triangle",
  }`
```

The mappings are a string because that is how they are passed to the javascript
code.

So far figuring out the button mappings has just been a process of putting some
sort of function in a browser console that tells you which button index is
pressed, connecting a gamepad and pressing each button individually and running
the function for each button. A function one could use would be:

```
f = () => {
  return navigator.getGamepads()[0].buttons.map((v, i) => {
    return {
      index: i,
      pressed: v.pressed
    }
  }).filter(v => v.pressed)[0].index
}
```

After writing this in the console one would press a button and run f() in the
console to find which index corresponds to that button.

Disclaimer: This is a very simple function with plenty of errors meant only for
this use case.

### Finding out if any requests need to be handled

When a controller is connected to a device there will typically be some back and
forth between the OS and the controller. For example, for both dualshock 4 and 3
the kernel will make a request to obtain the MAC address of the controller.
Since in these tests we are creating virtual controllers it is necessary to
replicate this back and forth with code.

This back and forth can come both from HID drivers and Chrome itself. There is
no formula to figure out what this communication would look like other than
diving yourself into the kernel/Chrome code or talking about it with someone who
is knowledgeable about the corresponding drivers or Chrome code who can walk you
through it.

After you have figured out what this back and forth looks like you can proceed to
handle it. The UHID interface deals with this communication by writing and
reading from the /dev/uhid file (the file that is represented by the
uhid.Device.File field). The tast uhid library allows the user to customize how
these requests are handled by providing an array of functions to which the user
can assign their own functions to handle requests.

The full list of requests that the UHID interface employs can be found at
https://www.kernel.org/doc/Documentation/hid/uhid.txt. As an example we'll be
lookgin at how the dualshock 3 test handles UHID_GET_REPORT requests.

```
func handleGetReportDS3(ctx context.Context, d *uhid.Device, buf []byte) error {
  // rnum is a field on the struct written to /dev/uhid that determines what
  // information is being requested.
	processRNum := func(rnum uhid.RNumType) ([]byte, error) {
    const (
      macAddressRequest uhid.RNumType = 0xf2
      operationalModeRequest = 0xf5
    )
    switch rnum {
    case macAddressRequest:
      // Array with mac address hardcoded into indexes 4-9, the rest is taken
      // from a real dualshock 3 reply to this request.
      return []byte{0xf2, 0xff, 0xff, 0x00, 0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0x00, 0x03, 0x40, 0x80, 0x18, 0x01, 0x8a}, nil
    case operationalModeRequest:
      // Once you know that no further requests are going to be made or that the
      // following requests don't need to be answered for proper functioning of
      // the device you can set this flag to true to allow the test to continue.
      jstest.KernelCommunicationDone = true
      return []byte{0x01, 0x00, 0x18, 0x5e, 0x0f, 0x71, 0xa4, 0xbb}, nil
    default:
      return []byte{}, errors.Errorf("unsupported request type: 0x%02x", rnum)
    }
  }
  
  // We read the struct written by the kernel to /dev/uhid, the buf field
  // contains the bytes already read from the file by the uhid library.
  reader := bytes.NewReader(buf)
  event := uhid.GetReportRequest{}
  if err := binary.Read(reader, binary.LittleEndian, &event); err != nil {
    return err
  }
  var data []byte
  var err error
  if data, err = processRNum(event.RNum); err != nil {
    return errors.Wrap(err, "failed parsing rnum in get report request")
  }
  
  // The ID of the reply has to be the same as the one in the request for the
  // kernel to be able to identify the reply.
  reply := uhid.GetReportReplyRequest{
    RequestType: uhid.GetReportReply,
    ID:          event.ID,
    Err:         0,
    DataSize:    uint16(len(data)),
  }
  copy(reply.Data[:], data[:])
  return d.WriteEvent(reply)
}
```

### Writing the test

After obtaining the recording and the button mappings and having written the
required handlers for your controller the rest of the test is simple.

Dualshock 3 test:

```
func DS3(ctx context.Context, s *testing.State) {
  const ds3HidRecording = "ds3.hid"
  d, err := jstest.CreateDevice(ctx, s.DataPath(ds3HidRecording))
  if err != nil {
    s.Fatal("Failed to create DS3: ", err)
  }
  s.Log("Created controller")
  // Uniq is a hardcoded field that stores the MAC address.
  d.SetUniq(dualshock.Uniq)
  d.EventHandlers[uhid.GetReport] = handleGetReportDS3
  expectedButtons := []string{
    "triangle",
    "circle",
    "x",
    "square",
  }
  mappings := `{
    0: "x",
    1: "circle",
    2: "square",
    3: "triangle",
  }`
  jstest.Gamepad(ctx, s, d, s.DataPath(ds3HidRecording), mappings, expectedButtons)
}
```

## Relevant links:

* hid-tools library: https://gitlab.freedesktop.org/libevdev/hid-tools
* uhid documentation: https://www.kernel.org/doc/Documentation/hid/uhid.txt
* uhid code: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/master:src/third_party/kernel/v4.4/include/uapi/linux/uhid.h
* uhid library one-pager: https://docs.google.com/document/d/1QfJoGl5lqThX0rtb4HRsARR0DV-XmNAzwwt5U6q4XLw/edit?ts=5e6fc8ac#heading=h.u9plwznjxhnu
