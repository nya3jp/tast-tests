// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	arcaudio "chromiumos/tast/local/bundles/cros/arc/audio"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioLoopbackCorrectness,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Plays sine wave with different config in ARC. Captures output audio via loopback and verifies the frequency of each channel",
		Contacts: []string{
			"chromeos-audio-bugs@google.com", // Media team
			"pteerapong@chromium.org",        // Author
		},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(
			// TODO(b/248994464) Test is flaky on low performance devices, mostly on octopus and dedede.
			// Skip the flaky boards until the issue is fixed.
			hwdep.SkipOnPlatform("octopus", "dedede"),
		),
		Fixture: "arcBooted",
		Attr:    []string{"group:mainline", "informational"},
		Timeout: 3 * time.Minute,
		Params: []testing.Param{
			{
				Name: "stereo_48000",
				Val: arcaudio.TestParameters{
					Class:         "org.chromium.arc.testapp.arcaudiotest.TestOutputSineActivity",
					SampleRate:    48000,
					ChannelConfig: arcaudio.ChannelConfigOutStereo,
				},
				ExtraSoftwareDeps: []string{"android_vm"},
			},

			// Test different sample rate
			{
				Name: "stereo_44100",
				Val: arcaudio.TestParameters{
					Class:         "org.chromium.arc.testapp.arcaudiotest.TestOutputSineActivity",
					SampleRate:    44100,
					ChannelConfig: arcaudio.ChannelConfigOutStereo,
				},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
			{
				Name: "stereo_32000",
				Val: arcaudio.TestParameters{
					Class:         "org.chromium.arc.testapp.arcaudiotest.TestOutputSineActivity",
					SampleRate:    32000,
					ChannelConfig: arcaudio.ChannelConfigOutStereo,
				},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
			{
				Name: "stereo_22050",
				Val: arcaudio.TestParameters{
					Class:         "org.chromium.arc.testapp.arcaudiotest.TestOutputSineActivity",
					SampleRate:    22050,
					ChannelConfig: arcaudio.ChannelConfigOutStereo,
				},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
			{
				Name: "stereo_16000",
				Val: arcaudio.TestParameters{
					Class:         "org.chromium.arc.testapp.arcaudiotest.TestOutputSineActivity",
					SampleRate:    16000,
					ChannelConfig: arcaudio.ChannelConfigOutStereo,
				},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
			{
				Name: "stereo_11025",
				Val: arcaudio.TestParameters{
					Class:         "org.chromium.arc.testapp.arcaudiotest.TestOutputSineActivity",
					SampleRate:    11025,
					ChannelConfig: arcaudio.ChannelConfigOutStereo,
				},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
			{
				Name: "stereo_8000",
				Val: arcaudio.TestParameters{
					Class:         "org.chromium.arc.testapp.arcaudiotest.TestOutputSineActivity",
					SampleRate:    8000,
					ChannelConfig: arcaudio.ChannelConfigOutStereo,
				},
				ExtraSoftwareDeps: []string{"android_vm"},
			},

			// Test different performance mode
			{
				Name: "stereo_48000_powersaving",
				Val: arcaudio.TestParameters{
					Class:           "org.chromium.arc.testapp.arcaudiotest.TestOutputSineActivity",
					SampleRate:      48000,
					ChannelConfig:   arcaudio.ChannelConfigOutStereo,
					PerformanceMode: arcaudio.PerformanceModePowerSaving,
				},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
			{
				Name: "stereo_48000_lowlatency",
				Val: arcaudio.TestParameters{
					Class:           "org.chromium.arc.testapp.arcaudiotest.TestOutputSineActivity",
					SampleRate:      48000,
					ChannelConfig:   arcaudio.ChannelConfigOutStereo,
					PerformanceMode: arcaudio.PerformanceModeLowLatency,
				},
				ExtraSoftwareDeps: []string{"android_vm"},
			},
		},
	})
}

// captureOutputAndGetFrequencies captures audio data and get the frequency stat of each channel.
func captureOutputAndGetFrequencies(ctx context.Context, output audio.TestRawData) ([]int, error) {
	if _, err := crastestclient.WaitForStreams(ctx, 5*time.Second); err != nil {
		return nil, errors.Wrap(err, "failed to wait for streams")
	}

	testing.ContextLog(ctx, "Capture output to ", output.Path)
	captureErr := crastestclient.CaptureFileCommand(
		ctx, output.Path,
		output.Duration,
		output.Channels,
		output.Rate).Run(testexec.DumpLogOnError)
	if captureErr != nil {
		return nil, errors.Wrap(captureErr, "capture data failed")
	}

	// Get frequency for each channel
	re := regexp.MustCompile("Rough   frequency:\\s+(-?\\d+)")
	var outputFreqs []int
	for channel := 1; channel <= output.Channels; channel++ {
		out, err := testexec.CommandContext(ctx, "sox",
			"-r", strconv.Itoa(output.Rate), // Sample rate
			"-b", strconv.Itoa(output.BitsPerSample), // Bits per sample
			"-c", strconv.Itoa(output.Channels), // Number of channels
			"-e", "signed", // Encoding type: signed integer
			"-t", "raw", // File type: raw
			output.Path,                    // Input file
			"-n",                           // Output file: Null
			"remix", strconv.Itoa(channel), // Extract audio from a specific channel
			"stat", // Get stat of the audio data
		).CombinedOutput(testexec.DumpLogOnError)
		if err != nil {
			return nil, errors.Wrapf(err, "sox stat failed on channel %d", channel)
		}

		freq := re.FindStringSubmatch(string(out))
		if freq == nil {
			testing.ContextLog(ctx, "sox stat: ", string(out))
			return nil, errors.Errorf("could not find frequency info from the sox result on channel %d", channel)
		}

		outputFreq, err := strconv.Atoi(freq[1])
		if err != nil {
			return nil, errors.Wrapf(err, "atoi failed on channel %d", channel)
		}

		outputFreqs = append(outputFreqs, outputFreq)
	}

	return outputFreqs, nil
}

// AudioLoopbackCorrectness plays sine wave with different config in ARC.
// Captures output audio via loopback and verifies the frequency of each channel.
func AudioLoopbackCorrectness(ctx context.Context, s *testing.State) {
	const (
		cleanupTime     = 30 * time.Second
		captureDuration = 1 // second(s)
		captureRate     = 48000
		freqTolerance   = 10

		keySampleRate      = "sample_rate"
		keyChannelConfig   = "channel_config"
		keyPerformanceMode = "perf_mode"
	)

	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Reserve time to remove input file and unload ALSA loopback at the end of the test.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to show Quick Settings: ", err)
	}
	defer quicksettings.Hide(cleanupCtx, tconn)

	// Defer this after deferring quicksettings.Hide to make sure quicksettings is still open when we
	// get the failure info.
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Set up capture (aloop) module.
	testing.ContextLog(ctx, "Setup aloop")
	unload, err := audio.LoadAloop(ctx)
	if err != nil {
		s.Fatal("Failed to load ALSA loopback module: ", err)
	}

	defer func(ctx context.Context) {
		// Wait for no stream before unloading aloop as unloading while there is a stream
		// will cause the stream in ARC to be in an invalid state.
		if err := crastestclient.WaitForNoStream(ctx, 5*time.Second); err != nil {
			s.Error("Wait for no stream error: ", err)
		}
		unload(ctx)
	}(cleanupCtx)

	// Select ALSA loopback output and input nodes as active nodes by UI.
	if err := quicksettings.SelectAudioOption(ctx, tconn, "Loopback Playback"); err != nil {
		s.Fatal("Failed to select ALSA loopback output: ", err)
	}
	// After selecting Loopback Playback, SelectAudioOption() sometimes detected that audio setting
	// is still opened while it is actually fading out, and failed to select Loopback Capture.
	// Call Hide() and Show() to reset the quicksettings menu first.
	quicksettings.Hide(ctx, tconn)
	quicksettings.Show(ctx, tconn)
	if err := quicksettings.SelectAudioOption(ctx, tconn, "Loopback Capture"); err != nil {
		s.Fatal("Failed to select ALSA loopback input: ", err)
	}

	testing.ContextLog(ctx, "Install app")
	if err := a.Install(ctx, arc.APKPath(arcaudio.Apk)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	defer a.Uninstall(cleanupCtx, arcaudio.Pkg)

	param := s.Param().(arcaudio.TestParameters)
	pkg := arcaudio.Pkg
	activityName := param.Class

	testing.ContextLogf(ctx, "Starting activity %s/%s", pkg, activityName)
	activity, err := arc.NewActivity(a, pkg, activityName)
	if err != nil {
		s.Fatalf("Failed to create activity %q in package %q: %v", activityName, pkg, err)
	}
	defer activity.Close()

	if err := activity.Start(ctx, tconn,
		arc.WithExtraIntUint64(keyPerformanceMode, uint64(param.PerformanceMode)),
		arc.WithExtraIntUint64(keySampleRate, param.SampleRate),
		arc.WithExtraIntUint64(keyChannelConfig, uint64(param.ChannelConfig))); err != nil {
		s.Fatalf("Failed to start activity %q in package %q: %v", activityName, pkg, err)
	}
	defer func(ctx context.Context) error {
		// Check that app is still running
		if _, err := ash.GetARCAppWindowInfo(ctx, tconn, activity.PackageName()); err != nil {
			return err
		}

		testing.ContextLogf(ctx, "Stopping activities in package %s", pkg)
		return activity.Stop(ctx, tconn)
	}(cleanupCtx)

	output := audio.TestRawData{
		Path:          filepath.Join(s.OutDir(), "audio_loopback_recorded.raw"),
		BitsPerSample: 16,
		Channels:      8, // Loopback module has 8 channels.
		Rate:          captureRate,
		Duration:      captureDuration,
	}
	outputFreqs, err := captureOutputAndGetFrequencies(ctx, output)
	if err != nil {
		s.Fatal("Failed to capture output: ", err)
	}

	var expectedFreqs []int
	switch param.ChannelConfig {
	case arcaudio.ChannelConfigOutStereo:
		expectedFreqs = []int{200, 500}
	case arcaudio.ChannelConfigOutQuad:
		expectedFreqs = []int{200, 300, 400, 500}
	case arcaudio.ChannelConfigOut5Point1:
		expectedFreqs = []int{200, 250, 400, 450, 300, 350}
	}

	for channel := 0; channel < len(expectedFreqs); channel++ {
		expectedFreq := expectedFreqs[channel]
		outputFreq := outputFreqs[channel]
		if math.Abs(float64(expectedFreq-outputFreq)) > freqTolerance {
			s.Errorf("channel %d frequency not matched. got: %d, expect: %d, tolerance: %d", channel+1, outputFreq, expectedFreq, freqTolerance)
		}
	}
}
