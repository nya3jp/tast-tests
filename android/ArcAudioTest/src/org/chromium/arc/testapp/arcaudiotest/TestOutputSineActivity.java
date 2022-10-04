// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package org.chromium.arc.testapp.arcaudiotest;

import android.content.res.AssetFileDescriptor;
import android.content.Intent;
import android.media.AudioFormat;
import android.media.AudioTrack;
import android.media.AudioAttributes;
import android.os.Bundle;
import android.util.Log;

/**
 * Activity for ChromeOS ARC++/ARCVM audio output sine test.
 */
public class TestOutputSineActivity extends MainActivity {
    private static final int DURATION = 4000; // 4 seconds

    private static final String KEY_ENCODING_FORMAT = "encoding_format";
    private static final String KEY_SAMPLE_RATE = "sample_rate";
    private static final String KEY_CHANNEL_CONFIG = "channel_config";
    private static final String KEY_PERFORMANCE_MODE = "perf_mode";

    private int mEncodingFormat = AudioFormat.ENCODING_PCM_16BIT;
    private int mSampleRate = 48000;
    private int mChannelConfig = AudioFormat.CHANNEL_OUT_STEREO;
    private int mPerformanceMode = AudioTrack.PERFORMANCE_MODE_NONE;

    private SamplePlayerThread mThread;

    /**
     * Starts the test by playing sine wave with provided config in a separate thread.
     */
    public void testPlaySine() {
        try {
            int[] freqs;
            switch(mChannelConfig) {
                case AudioFormat.CHANNEL_OUT_STEREO:
                    freqs = new int[]{200, 500};
                    break;
                case AudioFormat.CHANNEL_OUT_QUAD:
                    freqs = new int[]{200, 300, 400, 500};
                    break;
                case AudioFormat.CHANNEL_OUT_5POINT1:
                    freqs = new int[] {200, 250, 300, 350, 400, 450};
                    break;
                default:
                    Log.e(Constant.TAG,
                            String.format("Invalid channel config: %d. Aborting the app.",
                                    mChannelConfig));
                    finishAndRemoveTask();
                    return;
            }
            SamplePlayerSineGenerator player =
                new SamplePlayerSineGenerator(
                    mSampleRate,
                    mEncodingFormat,
                    mChannelConfig,
                    freqs
                );
            player.setPerformanceMode(mPerformanceMode);
            // Using any usage other than USAGE_MEDIA to prevent the performance mode from being
            // automatically change from PERFORMANCE_MODE_NONE to PERFORMANCE_MODE_POWER_SAVING
            // by AudioTrack.
            player.setAudioAttributes(new AudioAttributes.Builder()
                    .setUsage(AudioAttributes.USAGE_GAME)
                    .build());

            mThread = new SamplePlayerThread(player, DURATION);

            Log.d(Constant.TAG, "start playing sound");
            mThread.start();
        } catch (Exception e) {
            markAsFailed("testPlaySine failed. " + e.toString());
        }
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        // Get the message from the intent
        Intent intent = getIntent();
        mEncodingFormat = intent
                .getIntExtra(KEY_ENCODING_FORMAT, AudioFormat.ENCODING_PCM_16BIT);
        Log.i(Constant.TAG, String.format("Set mEncodingFormat: %d", mEncodingFormat));

        mSampleRate = intent
                .getIntExtra(KEY_SAMPLE_RATE, 48000);
        Log.i(Constant.TAG, String.format("Set mSampleRate: %d", mSampleRate));

        mChannelConfig = intent
                .getIntExtra(KEY_CHANNEL_CONFIG, AudioFormat.CHANNEL_OUT_STEREO);
        Log.i(Constant.TAG, String.format("Set mChannelConfig: %d", mChannelConfig));

        mPerformanceMode = intent
                .getIntExtra(KEY_PERFORMANCE_MODE, AudioTrack.PERFORMANCE_MODE_NONE);
        Log.i(Constant.TAG, String.format("Set PerformanceMode: %d", mPerformanceMode));
    }

    @Override
    protected void onStart() {
        super.onStart();
        Log.i(Constant.TAG, "start testPlaySine");
        testPlaySine();
        Log.i(Constant.TAG, "finish testPlaySine");
    }

    protected void onDestroy() {
        super.onDestroy();
        try {
            if(mThread != null) {
                mThread.stopPlayBack();
                mThread.join();
            }
        } catch (InterruptedException e) {
            e.printStackTrace();
        }
    }
}
