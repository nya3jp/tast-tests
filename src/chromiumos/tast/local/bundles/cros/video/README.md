# Video Test Overview ([tinyurl.com/cros-gfx-video](https://tinyurl.com/cros-gfx-video))

The Tast video tests are used to validate various video related functionality. A
wide range of test scenarios are available to validate both correctness and
performance. Some tests operate directly on top of the decoder and encoder
implementations, while other tests operate on the entire Chrome stack.

To get a list of all available video tests run:

    tast list $HOST | grep video.

All video tests can be found in the [tast video folder].

Some tests use Chrome while others do not; a hypothetical user would not see any
action in a DUT when running the latter.

[TOC]

## Capabilities and Capability Test

Tast can prevent running tests on SoCs without a particular functionality (see
[Test Dependencies], and the video tests make extensive use of this for gating
tests on hardware capabilities e.g. presence of video decoding for a given
codec. These capabilities are specified per-chipset (potentially with per-board
and per-device overlays) in files like e.g. [15-chipset-cml-capabilities.yaml].
These files are ingested in Go via the [`caps` package], so that test cases can
use them as preconditions in their `SoftwareDeps`.

For example: Intel Broadwell has support for hardware accelerated H.264 decoding
but not VP8. Therefore, the Broadwell associated capabilities file
[15-chipset-bdw-capabilities.yaml] has a number of `hw_dec_h264_*` entries and
no `hw_dec_vp8_*` entries. Tast ingests these capabilities file(s) and provides
them for tests with the name correspondence defined in the mentioned [`caps`
package]. Any test (case) with a `caps.HWDecodeH264` listed in its
`SoftwareDeps`, for example [`Play.h264_hw`], will thus run on Broadwell
devices, whereas those with `caps.HWDecodeVP8` listed will not, for example
[`Play.vp8_hw`].

Googlers can refer to go/crosvideocodec for more information about the hardware
supported video features.

The Capability test verifies that the capabilities provided by the YAML file(s)
are indeed detected by the hardware via the command line utility
`avtest_label_detect` that is installed in all test/dev images. This test can be
run by executing:

    tast run $HOST video.Capability

New boards being brought up will need to add capabilities files in the
appropriate locations for the Video Tast tests to run properly.

## Video decoder integration tests (`video.DecodeAccel`)

These tests validate video decoding functionality by running the
[video_decode_accelerator_tests]. They are implemented directly on top of the
video decoder implementations, not using Chrome. Various behaviors are tested
such as flushing and resetting the decoder. Decoded frames are validated by
comparing their checksums against expected values. For more information about
these tests check the [video decoder tests usage documentation].

Tests are available for various codecs such as H.264, VP8 and VP9. In addition
there are tests using videos that change resolution during plaback. To run all
tests use:

    tast run $HOST video.DecodeAccel.*

There are variants of these tests present that have 'VD' in their names. These
tests operate on the new video decoder implementations, which are set to replace
the current ones. To run all VD video decoder tests run:

    tast run $HOST video.DecodeAccelVD.*

### Video decoder performance tests (`video.DecodeAccelPerf`)

These tests measure video decode performance by running the
[video_decode_accelerator_perf_tests]. These tests are implemented directly on
top of the video decoder implementations and collect various metrics such as
FPS, CPU usage, power consumption (Intel devices only) and decode latency. For
more information about these tests check the
[video decoder performance tests usage documentation].

Performance tests are available for various codecs using 1080p and 2160p videos,
both in 30 and 60fps variants. To run all performance tests use:

    tast run $HOST video.DecodeAccelPerf.*

There are variants of these tests present that have 'VD' in their names. These
tests operate on the new video decoder implementations, which are set to replace
the current ones. To run all VD video decoder performance tests run:

    tast run $HOST video.DecodeAccelVDPerf.*

### Video decoder sanity checks

These tests use the [video_decode_accelerator_tests] to decode a video stream
with unsupported features. This is done by playing VP9 profile1-3 videos while
the decoder is incorrectly configured for profile0. The tests verify whether a
decoder is able to handle unexpected errors gracefully. To run all sanity checks
use:

    tast run $HOST video.DecodeAccelSanity.*

## Video encoder integration tests (`video.EncodeAccel`)

These tests run the [video_encode_accelerator_unittest] to test encoding raw
video frames. They are implemented directly on top of the video encoder
implementations, not using Chrome. Tests are available that test encoding H.264,
VP8 and VP9 videos using various resolutions.

To run all video encode tests use:

    tast run $HOST video.EncodeAccel.*

### Video encoder performance tests (`video.EncodeAccelPerf`)

These tests measure video encode performance by running the
[video_encode_accelerator_unittest]. They are implemented directly on top of the
video encoder implementations. Various metrics are collected such as CPU usage.
Tests are available for various codecs and resolutions. To run all tests use:

    tast run $HOST video.EncodeAccelPerf.*

## Play Tests (`video.Play`)

The Play test verifies whether video playback works by playing a video in the
Chrome browser. This test exercises the full Chrome stack, as opposed to others
that might only exercise the video decoder implementations. This test has
multiple variants, specified by the test case name parts. Every test case has a
codec part, e.g. `h264` or `av1` followed by none, one or several identifiers
(e.g. `hw` or `mse`).

- Cases without any identifier besides the codec name verify whether video
playback works by any means possible: fallback on a software video decoder is
allowed if hardware video decoding fails. These are basically the vanilla
variants/cases.

- Tests with a `guest` identifier use a guest ChromeOS login, versus those
without the identifier that use the normal test user login. Guest logins have a
different Chrome flag management.

- Tests with a `hw` identifier verify that hardware video decoding was used when
expected, as per the SoC capabilities (see
[Capabilities and Capability Test](#capabilities-and-capability-test)
Section), with fallback on a software
video decoder being treated as an error. Conversely, the tests with a `sw`
identifier force and verify the usage of a software video decoder, such as
libvpx or ffmpeg.

- Tests with an `mse` identifier use MSE (Media Source Extensions protocol) for
video playback, as opposed to those without the identifier that use a simple
file URL.

- Tests with the `alt` identifier employ an alternative hardware video decoding
implementation (see [tinyurl.com/chromeos-video-decoders](https://tinyurl.com/chromeos-video-decoders)).

To run these tests use:

    tast run $HOST video.Play.*

### Play Performance Tests (`video.PlaybackPerf`)

The video playback performance tests measure video decoder performance by
playing a video in the Chrome browser. These tests exercise the full Chrome
stack, as opposed to others that might only measure the performance of the
actual video decoder implementations. Both software and hardware video decoder
performance is measured. Various metrics are collected such as GPU usage, power
consumption and the number of dropped frames.

Tests are available for various codecs and resolutions, both in 30 and 60fps
variants. To run all tests use:

    tast run $HOST video.PlaybackPerf.*

## Seek Tests (`video.Seek`)

These tests verify seeking in videos: Seeks are issued while playing a video in
Chrome, waiting for the `onseeked` event to be received. These tests come in
variants with the same taxonomy as described in the
[Play Tests](#play-tests) Section, and in addition:

- Tests with a `stress` suffix issue a much larger amount of seeks, and have a
much larger timeout.

- Tests with a `switch` suffix utilize a resolution-changing video as input, to
introduce further stress in the video decoder implementations.

To run all video seek tests run:

    tast run $HOST video.Seek.*

### Resolution Ladder Sequence Creation

The `smpte_bars_resolution_ladder.*` videos are generated using a combination of
gstreamer and ffmpeg scripts, concretely for VP8, VP9 and H.264 (AVC1),
respectively:

    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=320,height=240   ! vp8enc ! "video/x-vp8" ! webmmux ! filesink location=smpte00.vp8.webm;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=854,height=480   ! vp8enc ! "video/x-vp8" ! webmmux ! filesink location=smpte01.vp8.webm;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=1280,height=800  ! vp8enc ! "video/x-vp8" ! webmmux ! filesink location=smpte02.vp8.webm;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=1904,height=1008 ! vp8enc ! "video/x-vp8" ! webmmux ! filesink location=smpte03.vp8.webm;

    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=320,height=240   ! vp9enc ! "video/x-vp9" ! webmmux ! filesink location=smpte00.vp9.webm;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=854,height=480   ! vp9enc ! "video/x-vp9" ! webmmux ! filesink location=smpte01.vp9.webm;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=1280,height=800  ! vp9enc ! "video/x-vp9" ! webmmux ! filesink location=smpte02.vp9.webm;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=1904,height=1008 ! vp9enc ! "video/x-vp9" ! webmmux ! filesink location=smpte03.vp9.webm;

    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=320,height=240   ! x264enc ! video/x-h264,profile=baseline ! mp4mux ! filesink location=smpte00.mp4;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=854,height=480   ! x264enc ! video/x-h264,profile=main     ! mp4mux ! filesink location=smpte01.mp4;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=1280,height=800  ! x264enc ! video/x-h264,profile=baseline ! mp4mux ! filesink location=smpte02.mp4;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=1904,height=1008 ! x264enc ! video/x-h264,profile=high     ! mp4mux ! filesink location=smpte03.mp4;

The resolutions were chosen to be of different aspect ratios: `4:3`, `16:9`,
`16:10` and `17:9`, respectively.

Once the subsequences are generated, they are concatenated by adding them in a
text file which contents would then be e.g.

    file 'smpte00.mp4'
    file 'smpte01.mp4'
    file 'smpte02.mp4'
    file 'smpte03.mp4'
    file 'smpte02.mp4'
    file 'smpte01.mp4'
    file 'smpte00.mp4'

which is then used for `ffmpeg`, for example for the MPEG-4 output:

    ffmpeg -f concat -i files.mp4.txt -bsf:v "h264_metadata=level=auto" -c copy smpte.mp4

The line for VP8 and VP9 is similar, without the `-bsf:v`.

## Canvas Tests (`video.DrawOnCanvas`)

This group of tests verifies that a video (presumably decoded using hardware
acceleration) can be drawn onto a 2D canvas. These tests exercise the full
Chrome stack. This path is important because it's used to display video
thumbnails is the Camera Capture App (CCA).

The tests are mostly driven by [video-on-canvas.html] which loads a video,
starts playing it, draws it once onto a 2D canvas, and finally reads the color
of the four corners of the canvas to assert a specific value on them (within
some tolerance).

To avoid worrying about when exactly the video is drawn to the canvas, the test
videos must be generated from a single still image. For example, to generate the
360p H.264 test video, a PNG image was saved from GIMP and the following command
adapted from the [ffmpeg Slideshow docs] was used:

    ffmpeg -loop 1 -i rect-640x360.png -c:v libx264 -pix_fmt yuv420p -t 1 -profile:v baseline still-colors-360p.h264.mp4

The still image consists of four solid color rectangles (one in each quadrant of
the image). The colors were chosen arbitrarily under the assumption that those
colors are unlikely to correspond to artifacts (for example, a common artifact
is a green line).

To run these tests use:

    tast run $HOST video.DrawOnCanvas.*

## Contents Tests (`video.Contents`)

This group of tests verifies that a video (presumably decoded using hardware
acceleration) is displayed correctly in full screen mode.

The tests start playing a video, switch it to full screen mode, take a
screenshot (using the CLI tool) and analyze the captured image to check the
color of a few interesting pixels. The test videos are re-used from the
[Canvas tests](#canvas-tests).

The pixels we check are the centers of each of the four rectangles of the test
video and the four corners of the video (plus some padding to ignore some
artifacts which are acceptable).

If the color check at the centers fails, video playback is broken in a very
visible (read major) way.

If the color check at the centers succeeds, but the check at the corners fails,
the likely culprit is an incorrect rectangle or size in the Chrome compositing
pipeline.

To run these tests use:

    tast run $HOST video.Contents.*

[15-chipset-bdw-capabilities.yaml]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/master:src/overlays/chipset-bdw/chromeos-base/autotest-capability-chipset-bdw/files/15-chipset-bdw-capabilities.yaml?q=15-chipset-bdw-capabilities.yaml
[15-chipset-cml-capabilities.yaml]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/master:src/overlays/chipset-cml/chromeos-base/autotest-capability-chipset-cml/files/15-chipset-cml-capabilities.yaml?q=15-chipset-cml-capabilities.yaml
[`autotest-capability`]: https://chromium.googlesource.com/chromiumos/overlays/chromiumos-overlay/+/master/chromeos-base/autotest-capability-default/
[`caps` package]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/master:src/platform/tast-tests/src/chromiumos/tast/local/media/caps/caps.go
[tast video folder]: https://chromium.googlesource.com/chromiumos/platform/tast-tests/+/refs/heads/master/src/chromiumos/tast/local/bundles/cros/video/
[video_decode_accelerator_tests]: https://cs.chromium.org/chromium/src/media/gpu/video_decode_accelerator_tests.cc
[video decoder tests usage documentation]: https://chromium.googlesource.com/chromium/src/+/master/docs/media/gpu/video_decoder_test_usage.md
[video_decode_accelerator_perf_tests]: https://cs.chromium.org/chromium/src/media/gpu/video_decode_accelerator_perf_tests.cc
[video decoder performance tests usage documentation]: https://chromium.googlesource.com/chromium/src/+/master/docs/media/gpu/video_decoder_perf_test_usage.md
[video_encode_accelerator_unittest]: https://cs.chromium.org/chromium/src/media/gpu/video_encode_accelerator_unittest.cc
[`Play.h264_hw`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/master:src/platform/tast-tests/src/chromiumos/tast/local/bundles/cros/video/play.go;l=92?q=h264_hw&ss=chromiumos%2Fchromiumos%2Fcodesearch
[`Play.vp8_hw`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/master:src/platform/tast-tests/src/chromiumos/tast/local/bundles/cros/video/play.go;l=99?q=h264_hw&ss=chromiumos%2Fchromiumos%2Fcodesearch
[Test Dependencies]: https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/test_dependencies.md
[video-on-canvas.html]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/master:src/platform/tast-tests/src/chromiumos/tast/local/bundles/cros/video/data/video-on-canvas.html
[ffmpeg Slideshow docs]: https://trac.ffmpeg.org/wiki/Slideshow
