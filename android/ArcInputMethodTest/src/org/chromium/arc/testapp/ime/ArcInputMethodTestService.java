/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.ime;

import android.inputmethodservice.InputMethodService;
import android.view.View;
import android.view.inputmethod.InputConnection;
import android.widget.Button;

public class ArcInputMethodTestService extends InputMethodService implements View.OnClickListener {
    private Button button;

    @Override
    public View onCreateInputView() {
        View view = getLayoutInflater().inflate(R.layout.keyboard_view, null);
        button = view.findViewById(R.id.a_button);
        button.setOnClickListener(this);
        return view;
    }

    @Override
    public void onClick(View view) {
        InputConnection ic = getCurrentInputConnection();
        if (ic != null) {
            ic.commitText("a", 0);
        }
    }
}
