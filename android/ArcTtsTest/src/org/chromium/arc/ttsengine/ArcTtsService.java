/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.ttsengine;

import android.os.Environment;
import android.speech.tts.SynthesisCallback;
import android.speech.tts.SynthesisRequest;
import android.speech.tts.TextToSpeech;
import android.speech.tts.TextToSpeechService;

import java.io.File;
import java.io.FileNotFoundException;
import java.io.FileOutputStream;
import java.io.PrintStream;

/**
 * A text to speech engine that outputs the generated speech
 * to a file called "ttsoutput.txt" under Downloads.
 * {@link android.speech.tts.TextToSpeechService}.
 */
public class ArcTtsService extends TextToSpeechService {

    @Override
    public void onCreate() {
        super.onCreate();
    }

    @Override
    public void onDestroy() {
        super.onDestroy();
    }

    @Override
    protected String[] onGetLanguage() {
        return null;
    }

    @Override
    protected int onIsLanguageAvailable(String lang, String country, String variant) {
        if("eng".equals(lang))
         return TextToSpeech.LANG_AVAILABLE;

         return TextToSpeech.LANG_NOT_SUPPORTED;
    }

    @Override
    protected synchronized int onLoadLanguage(String lang, String country, String variant) {
          return TextToSpeech.LANG_AVAILABLE;
    }

    @Override
    protected void onStop() {
      return;
    }

    @Override
    protected synchronized void onSynthesizeText(SynthesisRequest request,
            SynthesisCallback callback) {
      final String text = request.getText().toLowerCase();
      final File logFile =
          new File(Environment.getExternalStoragePublicDirectory(
              Environment.DIRECTORY_DOWNLOADS), "ttsoutput.txt");
      try (PrintStream out = new PrintStream(new FileOutputStream(logFile, true))) {
          out.println(text);
      } catch (FileNotFoundException e) {
          throw new RuntimeException(e);
      }

      return;
    }
}
