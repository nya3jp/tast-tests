# Video Test Overview ([tinyurl.com/cros-gfx-video](https://tinyurl.com/cros-gfx-video))

The Tast video tests are used to validate various video related functionality. A
wide range of test scenarios are available to validate both correctness and
performance. Some tests operate directly on top of the decoder and encoder
implementations, while other tests operate on the entire Chrome stack.

To get a list of all available video tests run:

    tast list $HOST video.*

All video tests can be found in the [tast video folder](https://chromium.googlesource.com/chromiumos/platform/tast-tests/+/refs/heads/main/src/chromiumos/tast/local/bundles/cros/video/).

[TOC]

[tast video folder]:

## Test layering

Test can be conceptually organized by the level of the protocol stack that is
exercised and verified. The following diagram shows a simplified software stack
where the numbers refers to tests that interact with and verify the layers
underneath.

![Test layering diagram](test_layering.png)

1. **Full stack** tests verify the whole Chrome stack, usually via DevTools
protocol. ARC++ or other user-facing apps are conceptually at this level,
although they are not in the scope of the video test package.
  Examples are `video.(Contents|DrawOnCanvas|MemCheck|Play|PlaybackPerf|Seek)`.
  Tests at this level oftentimes inspect system state via direct access to the
  file system, e.g. `MemCheck` retrieves the memory use from the Kernel DRI
  debug interface.

2. **Chrome `//media`** test binaries verify [`media::VideoDecoder`] and
[`media::VideoEncodeAccelerator`] functionality on top of real hardware. These
are:

  - [`video_decode_accelerator_tests`] and
[`video_decode_accelerator_perf_tests`], wrapped by
`video.DecodeAccel`/`video.DecodeAccelPerf` and
`video.DecodeAccelVD`/`video.DecodeAccelVDPerf` respectively, see the [video
decoder integration tests](#Video-decoder-integration-tests) Section.

  - [`video_encode_accelerator_tests`], wrapped by
`video.EncodeAccel`/`video.EncodeAccelPerf`,  see the [video encoder
integration tests](#Video-encoder-integration-tests) Section. (These tests
superseded the older `video_encode_accelerator_unittest`).

3. and 4. **Platform** tests verify video acceleration at the vendor-API level,
e.g. the kernel or userspace library. These tests do not need Chrome and are
ideal for the first stages of a new platform bringup. They might just be
wrappers around upstream validation tests. Examples are
`video.(PlatformEncoding|PlatformVAAPIUnittests|V4L2)` and any other `Platform`-
prefixed test.

A hypothetical user would not see any action on the screen of a DUT when
running tests of level 2, 3 or 4.

[`media::VideoDecoder`]: https://source.chromium.org/chromium/chromium/src/+/main:media/base/video_decoder.h;l=23;drc=5db067a3dc38dac442279730580c608d1db5e709
[`media::VideoEncodeAccelerator`]: https://source.chromium.org/chromium/chromium/src/+/main:media/video/video_encode_accelerator.h;l=107;drc=2dac3a71bdfe771e07e887dffe68b91a587b0c19

### Direct Video Decoder

ChromeOS supports both a legacy Video Decoder and a new direct VideoDecoder, see
[tinyurl.com/chromeos-video-decoders](https://tinyurl.com/chromeos-video-decoders)
(except for a few legacy platforms that only support the legacy one).

Both the legacy and the direct decoders must coexist for some time, hence tests
at the Chrome //media level are present for both implementations, marking the
direct ones with a `VD` prefix (e.g. `video.DecodeAccel` and
`video.DecodeAccelVD`). Full stack tests are focused on the implementation being
currently shipped, while keeping a few test cases operating on the alternate
VideoDecoder. These variants have an `alt` suffix.

There are [Software Dependencies] (see the
[Capabilities](#Capabilities) Section) to gate Tast tests,
namely: `video_decoder_direct`, `video_decoder_legacy` to mark the implementation
shipped by default, and `video_decoder_legacy_supported`.

## Capabilities

Tast can skip running tests on SoCs without a particular functionality by
specifying its [Software Dependencies]. All video tests make extensive use of
this for gating tests on e.g. support for decoding a given codec, etc.

This restriction can be bypassed during development.  Pass `-checktestdeps=false`
to tast in order to do so:

    tast run -checktestdeps=false $HOST video.*

The full specification can be found in the [`autotest-capability-default`]
package but, essentially these capabilities are specified per-chipset
(potentially with per-board and per-device overlays) in files like e.g.
[15-chipset-cml-capabilities.yaml]. These files are ingested in Go via the
[`caps` package], so that test cases can use them as preconditions in their
`SoftwareDeps`.

For example: Intel Skylake has support for hardware accelerated VP8 decoding but
not VP9. Therefore, the Skylake associated capabilities file
[15-chipset-skl-capabilities.yaml] has a number of `hw_dec_vp8_*` entries but no
`hw_dec_vp9_*` (the "minus" symbol is misleading: it does not mean "disable" but
simply itemizes the capabilities). Tast ingests these capabilities file(s) and
provides them for tests with the name correspondence defined in the mentioned
[`caps` package]. Any test (case) with a `caps.HWDecodeVP8` listed in its
`SoftwareDeps`, for example [`Play.vp8_hw`], will thus run on Skylake devices,
whereas those with `caps.HWDecodeVP9` listed will not, for example
[`Play.vp9_hw`].

Googlers can refer to [go/crosvideocodec](http://go/crosvideocodec) for more
information about the video features support.

[15-chipset-skl-capabilities.yaml]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/overlays/chipset-skl/chromeos-base/autotest-capability-chipset-skl/files/15-chipset-skl-capabilities.yaml;drc=45644e03a37aa93bf61d36dfdf2dc292940918e9
[15-chipset-cml-capabilities.yaml]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/overlays/chipset-cml/chromeos-base/autotest-capability-chipset-cml/files/15-chipset-cml-capabilities.yaml;drc=ddc0b955b61ab659142c5e226e6ae17aac5860af
[`autotest-capability-default`]: https://chromium.googlesource.com/chromiumos/overlays/chromiumos-overlay/+/ec7f22ef7d96f4325319dd2b641d820a6fffc5cb/chromeos-base/autotest-capability-default/
[`caps` package]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/common/media/caps/caps.go;drc=0df001c7962506063c1d8ba6a1b0df11d093ed32
[`Play.vp8_hw`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/local/bundles/cros/video/play.go;l=208;drc=2b77f33de4b453d9f7b73de36b6af38d355a04c4
[`Play.vp9_hw`]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/local/bundles/cros/video/play.go;l=232;drc=2b77f33de4b453d9f7b73de36b6af38d355a04c4

### Capability Test (`video.Capability`)

The Capability test verifies that the capabilities provided by the YAML file(s)
are indeed detected by the hardware via the command line utility
`avtest_label_detect` that is installed in all test/dev images. This test can be
run by executing:

    tast run $HOST video.Capability

New boards being brought up will need to add capabilities files in the
appropriate locations for the Video Tast tests to run properly.

## Runtime Variables

The encode tests uncompress video files in order to create a local file
for processing.  Under nominal conditions these files should be removed after
the test has run.  These uncompressed files along with their descriptive .json
files can be left on the device by passing the `removeArtifacts` variable to tast:

    tast run -var=videovars.removeArtifacts=false $HOST video.EncodeAccel.*

## Video decoder integration tests (`video.DecodeAccel`)

These tests validate video decoding at the **Chrome `//media`** level for
several codecs and resolutions by running [`video_decode_accelerator_tests`].
Various behaviors are tested such as flushing and resetting the decoder. In
addition there are tests using videos that change resolution during
playback. Decoded frames are validated by comparing their checksums against
expected values.

The `DecodeAccelVD` tests utilize the direct VideoDecoder implementation (see
the [Direct Video Decoder](#direct-video-decoder) Section). To run the test use:

    tast run $HOST video.DecodeAccel.*
    tast run $HOST video.DecodeAccelVD.*

### Video decoder performance tests (`video.DecodeAccelPerf`)

Similarly, `video.DecodeAccelPerf` and `video.DecodeAccelVDPerf` measure
Chrome's video decode stack performance by running
[`video_decode_accelerator_perf_tests`]. Various metrics are collected such as
decode latency, FPS, CPU or power usage for various codecs and resolutions. To
run these tests use:

    tast run $HOST video.DecodeAccelPerf.*
    tast run $HOST video.DecodeAccelVDPerf.*

### Video decoder compliance tests (`video.DecodeCompliance`)

These tests validate video decoding compliance by running
[`video_decode_accelerator_tests`] with various video clips and
`--gtest\_filter=VideoDecoderTest.FlushAtEndOfStream`. Unlike DecodeAccel and
DecodeAccelVD tests, DecodeCompliance mostly targets specific codec features and
is primarily concerned with the correctness of the produced frames. Currently,
we only test AV1. To run the test use:

    tast run $HOST video.DecodeCompliance.av1*

Please see [data/test_vectors/README.md] for details about the video clips used
in this test.

[data/test_vectors/README.md]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/local/bundles/cros/video/data/test_vectors/README.md;drc=5173965e8343c6b11fcd2edf06f5700042136e9d

## Video encoder integration tests (`video.EncodeAccel`)

These tests verify encoding at the **Chrome `//media`** level for several codecs
and resolutions. `video.EncodeAccel` wrap the new
[`video_encode_accelerator_tests`].

To run all video encode tests use:

    tast run $HOST video.EncodeAccel.*

### Video encoder performance tests (`video.EncodeAccelPerf`)

Similarly, `video.EncodeAccelPerf` measure Chrome's video encode stack
performance. Various metrics are collected such as FPS, CPU or power usage for
various codecs and resolutions. To run all tests use:

    tast run $HOST video.EncodeAccelPerf.*

## PlatformV4L2 Tests (`video.PlatformV4L2`)

V4L2 is a kernel video acceleration API. It is implemented in the kernel
linuxtv sub-tree, and it's used in Chrome for ARM-based boards such as those
shipped by Rockchip, MediaTek, and Qualcomm, and for the companion accelerator
chip Kepler.

### V4L2 video compliance tests (`video.PlatformV4L2.decoder`)

These tests validate the Video4Linux2 video acceleration APIs, for both
input and output. They attempt to verify that all expected IOCTLs are
indeed supported.

To run these tests use:

    tast run $HOST video.PlatformV4L2.decoder

## Video acceleration (VA) API unit test (`video.PlatformVAAPIUnittest`)

This test runs the `test_va_api` GTest binary from the libva-test
package. It checks the libva API against the driver implementation. See
https://github.com/intel/libva-utils for more details.

To run this test use:

    tast run $HOST video.PlatformVAAPIUnittest

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

## Play DRM Tests ('video.PlayDRM)

The PlayDRM tests are for playing back HW DRM protected video and
verifying all the supported codec + encryption scheme combinations.

-The tests use MSE/EME w/ Shaka Player to playback the content. The content
has the first 5 seconds as clear and the rest of the content is
encrypted w/ the technique matching the test case. The clips are 16
seconds long.

-After playback is done, it goes fullscreen with the video and then takes
a screenshot to verify the vast majority of the screen is solid black.
With HW DRM, video screenshots will be solid black. It then also uses
DevTools to verify the video pipeline is set up in a way that maps to HW
DRM (i.e. video is encrypted, HW decoder is used and a decrypting
demuxer is not used).

-Tests are named with `CENCVersion_EncryptionScheme_Codec`

-`CENCVersion` is either cencv1 (full sample encryption) or cencv3 (subsamples),
cencv1 is only tested with h264 as that is the only codec that needs to support
it

-`EncryptionScheme` is either cbc (for cbcs) or ctr (for cenc)

-`Codec` currently supports hevc, vp9 or h264

To run these tests use:

    tast run $HOST video.PlayDRM.*

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
gstreamer and ffmpeg scripts, concretely for AV1, VP8, VP9, H.264 (AVC1) and
H.265 (HEVC) respectively:

    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,format=I420,width=320,height=240   ! av1enc ! video/x-av1,profile=main ! webmmux ! filesink location=smpte00.webm;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,format=I420,width=854,height=480   ! av1enc ! video/x-av1,profile=main ! webmmux ! filesink location=smpte01.webm;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,format=I420,width=1280,height=800  ! av1enc ! video/x-av1,profile=main ! webmmux ! filesink location=smpte02.webm;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,format=I420,width=1904,height=1008 ! av1enc ! video/x-av1,profile=main ! webmmux ! filesink location=smpte03.webm;

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

    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=320,height=240   ! x265enc ! video/x-h265,profile=main ! h265parse ! mp4mux ! filesink location=smpte00.mp4;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=854,height=480   ! x265enc ! video/x-h265,profile=main ! h265parse ! mp4mux ! filesink location=smpte01.mp4;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=1280,height=800  ! x265enc ! video/x-h265,profile=main ! h265parse ! mp4mux ! filesink location=smpte02.mp4;
    gst-launch-1.0 -e videotestsrc num-buffers=60 pattern=smpte100 ! timeoverlay ! video/x-raw,width=1904,height=1008 ! x265enc ! video/x-h265,profile=main ! h265parse ! mp4mux ! filesink location=smpte03.mp4;

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

The line for AV1, VP8 and VP9 is similar, without the `-bsf:v`.

## Canvas Tests (`video.DrawOnCanvas`)

This group of tests verifies that a video (presumably decoded using hardware
acceleration) can be drawn onto a 2D canvas. These tests exercise the full
Chrome stack. This path is important because it's used to display video
thumbnails is the Camera Capture App (CCA).

Each test loads a video, starts playing it, draws it once onto a 2D canvas, and
finally reads a few interesting pixels to compare the result against a reference
image. The comparison is done in the same fashion as in the
[Contents tests](#contents-tests).

Currently, these tests report the color distance for each sampled pixel as a
performance measurement. They don't currently fail due to unexpected colors
except if anything is drawn outside of the expected bounds.

The test videos are designed with certain considerations in mind:

- They consist of a single still image so that the exact moment at which we
capture its pixels doesn't matter.
- We have cases where the video's visible size is the same as its coded size
(e.g., a 1280x720 H.264) and cases where it's not (e.g., a 640x360 H.264).
- In cases where the visible size is not the same as the coded size, the
non-visible area contains a color that's unexpected by the test so that certain
cases of incorrect cropping can be detected.
- We have a case where the visible rectangle does not start at (0, 0). These
videos are not expected to be common in the wild, but the H.264 specification
allows such exotic crops.
- The videos are long enough that we don't have to worry about the possibility
of capturing an empty frame when the video loops.

See [this](#generation-of-test-videos) for the script used to generate these
videos.

To run these tests use:

    tast run $HOST video.DrawOnCanvas.*

[video-on-canvas.html]: https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/tast-tests/src/chromiumos/tast/local/bundles/cros/video/data/video-on-canvas.html;drc=745a69342e7a7fa5a6bb1e9bc504b6d0dbb17d92

## Contents Tests (`video.Contents`)

This group of tests verifies that a video (presumably decoded using hardware
acceleration) is displayed correctly in full screen mode.

The tests start playing a video, switch it to full screen mode, take a
screenshot (using the CLI tool) and analyze the captured image to check the
color of a few interesting pixels. The test videos are re-used from the
[Canvas tests](#canvas-tests).

There are two variations: *_hw and *_composited_hw. The *_hw tests only run on
devices that support NV12 overlays (see `hwdep.SupportsNV12Overlays()`. The
*_composited_hw tests don't have this restriction: they force hardware overlays
off so that they have to be composited.

Additionally, there are lacros variations for the *_exotic_crop_hw and
the *_exotic_crop_composited_hw tests. Currently, both lacros-chrome and
ash-chrome can demote videos to non-overlay by using their corresponding
--enable-hardware-overlays flag. Both paths handle video visible rectangles
differently. Therefore, the composited test for lacros needs two variations:
one where only lacros-chrome disables overlays (*_lacros_composited_hw_lacros)
and one where only ash-chrome disables overlays (*_ash_composited_hw_lacros).

To check for color correctness, we sample a few interesting pixels. A video
frame should have the following structure:

```
*MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM*
M-MM-MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM-MM-M
MMMAAAAAAAAAAAAAAAAAAAAAAABBBBBBBBBBBBBBBBBBBBBBBMMM
M-MA-AAAAAAAAAAAAAAAAAAAAABBBBBBBBBBBBBBBBBBBBB-BM-M
MMMAAAAAAAAAAAAAAAAAAAAAAABBBBBBBBBBBBBBBBBBBBBBBMMM
MMMAAAAAAAAAAAAAAAAAAAAAAABBBBBBBBBBBBBBBBBBBBBBBMMM
MMMAAAAAAAAAAAAAAAAAAAAAAABBBBBBBBBBBBBBBBBBBBBBBMMM
MMMCCCCCCCCCCCCCCCCCCCCCCCDDDDDDDDDDDDDDDDDDDDDDDMMM
MMMCCCCCCCCCCCCCCCCCCCCCCCDDDDDDDDDDDDDDDDDDDDDDDMMM
MMMCCCCCCCCCCCCCCCCCCCCCCCDDDDDDDDDDDDDDDDDDDDDDDMMM
M-MC-CCCCCCCCCCCCCCCCCCCCCDDDDDDDDDDDDDDDDDDDDD-DM-M
MMMCCCCCCCCCCCCCCCCCCCCCCCDDDDDDDDDDDDDDDDDDDDDDDMMM
M-MM-MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM-MM-M
*MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM*
```

Where M is a magenta border; A, B, C, and D are the colors of the quadrants;
and - and * are the pixels we sample. * are the "outer corners" and - are the
"inner corners." Note that for the inner corners, we sample four pixels per
corner: three of those inside the magenta border and one inside the quadrant.
These pixels are near each other, so this helps us detect incorrect
stretching/shifting/rotation/mirroring. Sampling the outer corners helps us
detect leakage of invisible data. In practice, we offset three of the outer
corner sampling points by 1px in video frame space in order to ignore expected
color blending artifacts in the "exotic crop" cases where there is invisible
data all around the visible area of the video. For the outer bottom-right
corner, we don't offset the sampling point: we never expect blending artifacts
there, even for the exotic crop cases.

Currently, these tests report the color distance for each sampled pixel as a
performance measurement. They don't currently fail due to unexpected colors.

To run these tests use:

    tast run $HOST video.Contents.*

## MemCheck Tests (`video.MemCheck`)

The full stack `MemCheck` tests verify that there are no memory leaks while
playing videos. Two different variants are provided:

- Tests with a simple codec identifier (e.g. `vp8_hw`) operate by monitoring the
amount of Framebuffers of a specific resolution increases during playback and
then returns to the original count after playback is finished. These
Framebuffers  correspond to the amount of allocated video frame resources.

- Tests with a `switch` suffix play back several resolutions cyclically using
MSE while verifying that the amount of Framebuffers of any of a given set of
resolutions never exceeds a given value.

## Addendum

### Generation of test videos (for `video.{DrawOnCanvas,Contents}`)

```
#!/bin/bash

# Generates an image of size canvas_width x canvas_height. The image contains an
# area of size area_width x area_height starting at (offset_x, offset_y) which
# has an 8px magenta interior border and the remaining area is subdivided into
# quadrants. Each quadrant is colored differently. The rest of the image is
# cyan. The colors are chosen so that the distance between any two colors is at
# least 127 (when taking the maximum absolute difference between components).
# That way, it's easy to programmatically distinguish among regions. Also, the
# assumption is that those colors are unlikely to correspond to common artifacts
# (e.g., green lines). The image is saved as output_file. An image of only
# area_width x area_height starting at (offset_x, offset_y) is saved as
# output_ref_file if provided: this is intended to be used as the "reference"
# image for how a video frame generated from output_file should be rendered in
# the absence of scaling and artifacts.
gen_image() {
  canvas_width=$1
  canvas_height=$2
  area_width=$3
  area_height=$4
  offset_x=$5
  offset_y=$6
  output_file=$7
  output_ref_file=$8

  # Calculate the coordinates of the top left corner of the area (x0, y0), the
  # middle of the area (x50, y50), and the bottom right corner (x100, y100).
  ((x0 = offset_x))
  ((y0 = offset_y))
  ((x50 = offset_x + area_width / 2 - 1))
  ((y50 = offset_y + area_height / 2 - 1))
  ((x100 = offset_x + area_width - 1))
  ((y100 = offset_y + area_height - 1))

  # Draw rectangles in the following order: top-left, top-right, bottom-right,
  # bottom-left.
  convert -size ${canvas_width}x${canvas_height} canvas:cyan -draw " \
    fill rgba(255, 0, 255) \
    rectangle ${x0},${y0} ${x100},${y100} \
    fill rgba(128, 128, 0, 255) \
    rectangle $((x0 + 8)),$((y0 + 8)) ${x50},${y50} \
    fill rgba(0, 128, 128, 255) \
    rectangle $((x50 + 1)),$((y0 + 8)) $((x100 - 8)),${y50} \
    fill rgba(128, 0, 128, 255) \
    rectangle $((x50 + 1)),$((y50 + 1)) $((x100 - 8)),$((y100 - 8)) \
    fill rgba(0, 0, 128, 255) \
    rectangle $((x0 + 8)),$((y50 + 1)) ${x50},$((y100 - 8))" \
    ${output_file}

  if [ -n "$output_ref_file" ]
  then
    convert ${output_file} -crop ${area_width}x${area_height}+${x0}+${y0} \
      +repage ${output_ref_file}
    exiftool -overwrite_original -all= ${output_ref_file}
  fi
}

# Given an image (input_file), generates an H.264 video which lasts 30 seconds,
# removes the EXIF metadata, and saves it as output_file.
gen_video() {
  input_file=$1
  output_file=$2
  ffmpeg -y -loop 1 -i ${input_file} -c:v libx264 -pix_fmt yuv420p -t 30 \
    -profile:v baseline ${output_file}
  exiftool -overwrite_original -all= ${output_file}
}

# Similar to gen_video(), but we get to specify the H.264 crop rectangle.
gen_cropped_video() {
  crop_top=$1
  crop_right=$2
  crop_bottom=$3
  crop_left=$4
  input_file=$5
  output_file=$6
  ffmpeg -y -loop 1 -i ${input_file} -c:v libx264 -pix_fmt yuv420p -t 30 \
    -profile:v baseline \
    -bsf:v h264_metadata="crop_top=${crop_top}: \
                          crop_right=${crop_right}: \
                          crop_bottom=${crop_bottom}: \
                          crop_left=${crop_left}" \
    ${output_file}
  exiftool -overwrite_original -all= ${output_file}
}

# Modifies file (MP4) to make sure the image width and source image width
# reported by exiftool is width. The offsets used here assume that the EXIF
# metadata has been removed from the file.
overwrite_image_width() {
  file=$1
  width=$2
  echo "000000f8: $(printf "%04x" ${width})" | xxd -r - ${file}
  echo "000001f1: $(printf "%04x" ${width})" | xxd -r - ${file}
}

# Same as overwrite_image_width() but for the height.
overwrite_image_height() {
  file=$1
  height=$2
  echo "000000fc: $(printf "%04x" ${height})" | xxd -r - ${file}
  echo "000001f3: $(printf "%04x" ${height})" | xxd -r - ${file}
}

################################################################################
# Generate typical videos:
#
# The visible rectangle for these videos starts at (0, 0). For some of these
# videos, the coded size is different from the visible size because the coded
# size is aligned to 16 on each dimension. For example, for a 640x360 H.264
# video, the coded size is 640x368. If we supplied a 640x360 image, ffmpeg will
# repeat the last row to fill the remaining 640x8 pixels prior to encoding. This
# is not desirable because if Chrome doesn't apply the visible rectangle for
# cropping (something that has occurred in the past) video.Contents and
# video.DrawOnCanvas will have a hard time detecting that regression because of
# the way the color at the edges are checked. Instead, we give ffmpeg a 640x368
# image where the last 640x8 are of a color not expected by the test. That way,
# ffmpeg doesn't have to pad. When we do this, we also have to tell ffmpeg the
# H.264 crop rectangle and modify the resulting MP4 file to make sure it carries
# a 640x360 size instead of 640x368.

gen_image 640 368 640 360 0 0 still-colors-360p.bmp \
  still-colors-360p.ref.png
gen_cropped_video 0 0 8 0 still-colors-360p.bmp still-colors-360p.h264.mp4
overwrite_image_height still-colors-360p.h264.mp4 360

gen_image 864 480 854 480 0 0 still-colors-480p.bmp \
  still-colors-480p.ref.png
gen_cropped_video 0 10 0 0 still-colors-480p.bmp still-colors-480p.h264.mp4
overwrite_image_width still-colors-480p.h264.mp4 854

gen_image 1280 720 1280 720 0 0 still-colors-720p.bmp \
  still-colors-720p.ref.png
gen_video still-colors-720p.bmp still-colors-720p.h264.mp4

gen_image 1920 1088 1920 1080 0 0 still-colors-1080p.bmp \
  still-colors-1080p.ref.png
gen_cropped_video 0 0 8 0 still-colors-1080p.bmp still-colors-1080p.h264.mp4
overwrite_image_height still-colors-1080p.h264.mp4 1080

################################################################################
# Generate a video with an exotic visible rectangle:
#
# H.264 allows for fancy visible rectangles that don't start at (0, 0). The
# video here is 720x480 but it is cropped by a different amount on each side
# (using H.264 metadata) in such a way that the visible area ends up being
# 640x360 (thus, the resulting video should be rendered essentially like
# still-colors-360p.h264.mp4 above).

gen_image 720 480 640 360 64 32 still-colors-720x480.bmp
gen_cropped_video 32 16 88 64 still-colors-720x480.bmp \
  still-colors-720x480-cropped-to-640x360.h264.mp4
```

[`video_decode_accelerator_tests`]: https://chromium.googlesource.com/chromium/src/+/046f987e020baba45ffb3061b3ee3d960d6ce981/docs/media/gpu/video_decoder_test_usage.md
[`video_decode_accelerator_perf_tests`]: https://chromium.googlesource.com/chromium/src/+/046f987e020baba45ffb3061b3ee3d960d6ce981/docs/media/gpu/video_decoder_perf_test_usage.md
[`video_encode_accelerator_tests`]: https://chromium.googlesource.com/chromium/src/+/046f987e020baba45ffb3061b3ee3d960d6ce981/docs/media/gpu/video_encoder_test_usage.md
[Software Dependencies]: https://chromium.googlesource.com/chromiumos/platform/tast/+/HEAD/docs/test_dependencies.md
