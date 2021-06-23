/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.tts;

import android.os.Environment;
import android.speech.tts.SynthesisCallback;
import android.speech.tts.SynthesisRequest;
import android.speech.tts.TextToSpeech;
import android.speech.tts.TextToSpeechService;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;

/**
 * A text to speech engine that outputs the generated speech to a file called "ttsoutput.txt" under
 * Downloads. {@link android.speech.tts.TextToSpeechService}.
 */
public class ArcTtsService extends TextToSpeechService {
    final static String OUTPUT_FILENAME = "ttsoutput.txt";

    @Override
    protected String[] onGetLanguage() {
        return new String[] {"eng", "USA", ""};
    }

    @Override
    protected int onIsLanguageAvailable(String lang, String country, String variant) {
        if (lang.equals("eng")) return TextToSpeech.LANG_AVAILABLE;
        return TextToSpeech.LANG_NOT_SUPPORTED;
    }

    @Override
    protected synchronized int onLoadLanguage(String lang, String country, String variant) {
        if (lang.equals("eng")) return TextToSpeech.LANG_AVAILABLE;
        return TextToSpeech.LANG_NOT_SUPPORTED;
    }

    @Override
    protected void onStop() {
        return;
    }

    @Override
    protected synchronized void onSynthesizeText(
            SynthesisRequest request, SynthesisCallback callback) {
        Path path =
                Paths.get(
                        Environment.getExternalStoragePublicDirectory(
                                        Environment.DIRECTORY_DOWNLOADS)
                                .getPath(),
                        OUTPUT_FILENAME);

        callback.done();

        try {
            Files.createFile(path);
            byte[] bytes = request.getText().getBytes();
            Files.write(path, bytes, java.nio.file.StandardOpenOption.APPEND);
        } catch (IOException e) {
            throw new RuntimeException(e);
        }
        return;
    }
}
