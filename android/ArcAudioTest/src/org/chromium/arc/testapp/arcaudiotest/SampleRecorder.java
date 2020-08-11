// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package org.chromium.arc.testapp.arcaudiotest;

import android.media.AudioRecord;
import android.media.MediaRecorder;
import android.util.Log;
import java.io.File;
import java.io.FileOutputStream;
import java.nio.Buffer;
import java.nio.ByteBuffer;

public class SampleRecorder {

    private final int mSampleRate;
    private final int mEncoding;
    private final int mChannelConfig;
    private final int mBufferSize;
    protected AudioRecord mRecord;

    SampleRecorder(int sampleRate, int encoding, int channelConfig) {
        mSampleRate = sampleRate;
        mEncoding = encoding;
        mChannelConfig = channelConfig;
        mBufferSize = AudioRecord.getMinBufferSize(mSampleRate, mChannelConfig, mEncoding) * 3;
        mRecord =
            new AudioRecord(
                MediaRecorder.AudioSource.DEFAULT,
                mSampleRate,
                mChannelConfig,
                mEncoding,
                mBufferSize);
        Log.i(
            Constant.TAG,
            String.format(
                "recorder created using rate = %d, encoding = 0x%08x",
                mSampleRate, mEncoding));
    }

    public int getAudioSessionId() {
        return mRecord.getAudioSessionId();
    }

    public void record(File file) throws Exception {
        final long TEST_DURATION_MILLIS = 5000;
        final ByteBuffer buffer = ByteBuffer.allocateDirect(mBufferSize);

        mRecord.startRecording();
        try (FileOutputStream outStream = new FileOutputStream(file)) {
            long elapsedMillis = 0;
            long startTime = System.currentTimeMillis();
            while (elapsedMillis < TEST_DURATION_MILLIS) {
                int result = mRecord.read(buffer, mBufferSize);
                if (result < 0) {
                    throw new RuntimeException("Reading of audio buffer failed: " + result);
                }
                Log.i(Constant.TAG, String.format("recorder read %d bytes", result));
                outStream.write(buffer.array(), 0, result);
                elapsedMillis = System.currentTimeMillis() - startTime;
                ((Buffer) buffer).clear();
            }
        } catch (Exception e) {
            throw new RuntimeException("Writing of recorded audio failed", e);
        } finally {
            mRecord.stop();
            Log.i(Constant.TAG, "mRecord.stop()");
        }
    }

    public void release() {
        mRecord.release();
        Log.i(Constant.TAG, "mRecord.release()");
    }
}
