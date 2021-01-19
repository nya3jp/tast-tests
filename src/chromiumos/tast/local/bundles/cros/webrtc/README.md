# WebRTC Tests Overview ([tinyurl.com/cros-gfx-webrtc](https://tinyurl.com/cros-gfx-webrtc))

The Tast WebRTC tests are used to exercise and verify various WebRTC-related
APIs. WebRTC in the current scope refers to the W3C Specifications (Specs)
published under the [Web Real-Time Communications Working Group], for use by a
Web Application and provided by a User Agent (in this case Chrome). C++, Java or
other built-in APIs can be derived from these ECMAScript-based Specs, and are
beyond the scope of this document and test cases. Some popular Specifications
under the WebRTC umbrella are:

- [WebRTC 1.0: Real-time Communication Between Browsers], defining APIs for
media to be sent and received between possibly remote browsers. This includes
the `RTCPeerConnection` interface, which supports peer-to-peer communications.

- [Media Capture and Streams], defining APIs for accessing capture devices such
as webcams and microphones via the popular `getUserMedia()` interface. This Spec
also defines the concepts of `MediaStream` and `MediaStreamTrack` that are used
as opaque connectors for all WebRTC APIs.

  - The companion [Screen Capture] Spec defines the API for capturing content
such as the screen, a window or an application using the `getDisplayMedia()`
API.

- [MediaStream Recording], defining APIs for real-time encoding of media data
flowing along a `MediaStream`. See the Chromium [README.md] for some important
implementation notes.

The tests in this [Tast webrtc folder] are based on the mentioned API names. To
get a list of all available WebRTC tests run:

    tast list $HOST | grep webrtc.

[TOC]

## WebRTC for video calls

WebRTC defines many components to build real-time applications that can be
purely local (e.g. a microphone or screen recorder) or remote (e.g. a video call
service or a live streaming application). Indeed,  WebRTC provides support for
popular video/audio call services such as Google Meet, Slack, Microsoft Teams
and Zoom.

A popular WebRTC use case is multi-party video calls; these can be implemented
as multiple concurrent peer-to-peer connections, but as the number of
participants grows it's inefficient to have multiple such individual streams
because of the associated amount of encoder and decoder instances. It's common
then to use a central server, either a Selective Forwarding Unit (SFU) or a
Multipoint Control Unit (MCU), depending respectively on whether it forwards
streams of data untouched or modifies them, usually decoding and reencoding
them. SFUs are very popular due to their lower computational complexity. SFU
clients are usually configured to send several encoded versions of the same
video feed with varying resolution, frame rate and encoding quality. Each of
these sub streams is called a layer and can be selected and forwarded more or
less independently by the SFU. For example, a given client might be sending at a
given time the same video feed encoded as 640x360 and 320x180 pixels, enabling
the SFU to decide which one to forward to remote peers. The mandatory codecs in
WebRTC, VP8 and H.264 (see [RFC7742]), use `simulcast` for layering, whereas
later codecs such as VP9 and AV1 use `Scalable Video Coding` (SVC). For more
information see [here](https://webrtchacks.com/sfu-simulcast/) and
[here](https://webrtchacks.com/chrome-vp9-svc/). Sophisticated video
conferencing services such as Google Meet/Hangouts make extensive use of
layering.

## WebRTC in Chrome

Chrome's WebRTC implementation resides in `//third_party/webrtc` which is a
mirror of https://webrtc.googlesource.com/src. A large part of the functionality
runs in Chrome's Renderer process, and concretely inside the Blink rendering
engine, with hooks to delegate certain functionality such as video/audio
capture/encoding/decoding or network I/O to other parts of Chrome (see [WebRTC
architecture]). Of particular interest for ChromeOS is the offloading of video
encoding and decoding to hardware accelerators, when those are available; the
verification of this functionality is a primary concern of the tests in this
folder.

WebRTC-in-Chrome supports a series of codecs or codec profiles, e.g. for video
it supports VP9 and the mandatory VP8 and H.264. The actual codec to be used is
decided during the establishment of a remote peer connection in an
implementation-dependent process beyond the scope of this text, but it's
reasonable to assume that hardware accelerated codecs are given priority. Note
that WebRTC-in-Chrome must nearly always have a software encoder/decoder
fallback per accelerator.

## RTC PeerConnection tests (`webrtc.RTCPeerConnection`)

The RTC PeerConnection test verifies whether peer connections work correctly by
establishing a loopback call, i.e. a call between two tabs in the same machine.
This test has multiple variants, specified by the test case name parts. Every
test case has a codec part, e.g. `h264` or `vp9` followed by none, one or
several identifiers (e.g. `enc`).

- Cases without any identifier besides the codec name verify whether
RTCPeerConnection works by any means possible: fallback on a software video
decoder/encoder is allowed if hardware video decoding/encoding fails. These are
basically the vanilla variants/cases.

- Tests with an `enc` identifier verify that hardware video encoding was used
when expected, as per the SoC capabilities (see [Video Capabilities]), with a
fallback on a software video encoder being treated as an error. Conversely, the
tests with a `dec` identifier force and verify the usage of a hardware video
decoder.

- Tests with a `simulcast` identifier establish the loopback connection
specifying use of VP8 simulcast with several layers (see the [WebRTC for video
calls](#webrtc-for-video-calls) Section); the `enc_simulcast` identifier
indicates that the use of a hardware video encoder is forced and verified.

- Tests with the `cam` identifier utilize the internal DUT camera if this is
available, verifying that the format produced by it is compatible with the
hardware video encoding stack. Tests without this identifier use the [fake video
capture device].

To run these tests use:

    tast run $HOST webrtc.RTCPeerConnection.*

### RTC PeerConnection Perf tests (`webrtc.RTCPeerConnectionPerf`)

The RTC PeerConnection Perf tests collect various performance metrics while
running a loopback connection similar to the `webrtc.RTCPeerConnection` tests.
Like those, multiple variants are identified primarily by a codec name in the
test case name and an extra "hw" or "sw" identifier to indicate the encoding and
decoding implementation used. Some of the collected metrics are GPU usage, power
consumption and average encoding and decoding time.

- Tests with a `multi_..._NxN` identifier establish the loopback connection
as the rest of the test case identifiers would determine, and then play in
parallel as many videos of the specified codec so as to have an NxN grid, scaling
the videos as necessary.

## MediaRecorder tests (`webrtc.MediaRecorder`)

`MediaRecorderAccelerator` verifies that `MediaRecorder` uses the video hardware
encoding accelerator if this is expected from the device capabilities. The test
cases are divided by codec similar to the [RTC PeerConnection
tests](#rtc-peerconnection-tests).

- Tests with the `cam` identifier utilize the internal DUT camera if this is
available, verifying that the format produced by it is compatible with the
hardware video encoding stack. Tests without this identifier use the [fake video
capture device].

`MediaRecorder` is a legacy test that verifies the MediaRecorder JS API
functionality.

### MediaRecorder Perf tests (`webrtc.MediaRecorderPerf`)

The MediaRecorder Perf tests collect various performance metrics while running a
recording session for a given amount of time. Like other tests in this folder,
multiple variants are identified primarily by a codec name in the test case name
and an extra "hw" or "sw" identifier to indicate the encoding and decoding
implementation used and verified. Some of the collected metrics are, again, GPU
usage, power consumption and average encoding and decoding time.

## GetDisplayMedia tests (`webrtc.GetDisplayMedia`)

GetDisplayMedia is the JS API used for screen/window/tab content capture. The
test cases here implemented verify that some of these sources, identified by
their specification names, can indeed capture content and do not produce black
frames.


[Web Real-Time Communications Working Group]: https://www.w3.org/groups/wg/webrtc/publications
[WebRTC 1.0: Real-time Communication Between Browsers]: https://www.w3.org/TR/webrtc/
[Media Capture and Streams]: https://www.w3.org/TR/mediacapture-streams/
[Screen Capture]: https://www.w3.org/TR/screen-capture/
[MediaStream Recording]: https://www.w3.org/TR/mediastream-recording/
[README.md]: https://chromium.googlesource.com/chromium/src/+/master/third_party/blink/renderer/modules/mediarecorder/README.md
[RFC7742]: https://tools.ietf.org/html/rfc7742#section-5
[Tast webrtc folder]: https://chromium.googlesource.com/chromiumos/platform/tast-tests/+/refs/heads/main/src/chromiumos/tast/local/bundles/cros/webrtc/
[WebRTC architecture]: http://webrtc.github.io/webrtc-org/architecture/#
[fake video capture device]: http://webrtc.github.io/webrtc-org/testing/
[Video Capabilities]: https://chromium.googlesource.com/chromiumos/platform/tast-tests/+/HEAD/src/chromiumos/tast/local/bundles/cros/video/README.md#capabilities-and-capability-test
