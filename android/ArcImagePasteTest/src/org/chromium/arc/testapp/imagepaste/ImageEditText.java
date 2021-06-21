/*
 * Copyright 2021 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.imagepaste;

import android.content.Context;
import android.net.Uri;
import android.os.Bundle;
import android.util.AttributeSet;
import android.view.inputmethod.EditorInfo;
import android.view.inputmethod.InputConnection;
import android.view.inputmethod.InputConnectionWrapper;
import android.view.inputmethod.InputContentInfo;
import android.widget.EditText;

public class ImageEditText extends EditText {
    interface Listener {
        void onContentUri(Uri uri);
    }

    private Listener mListener;

    void setListener(Listener listener) { mListener = listener; }

    public ImageEditText(Context context) {
        super(context);
    }

    public ImageEditText(Context context, AttributeSet attrs) {
        super(context, attrs);
    }

    public ImageEditText(Context context, AttributeSet attrs, int defStyleAttr) {
        super(context, attrs, defStyleAttr);
    }

    public ImageEditText(Context context, AttributeSet attrs, int defStyleAttr, int defStyleRes) {
        super(context, attrs, defStyleAttr, defStyleRes);
    }

    @Override
    public InputConnection onCreateInputConnection(EditorInfo editorInfo) {
        return new InputConnectionWrapper(super.onCreateInputConnection(editorInfo), false) {
            @Override
            public boolean commitContent(
                    InputContentInfo inputContentInfo, int flags, Bundle opts) {
                inputContentInfo.requestPermission();
                try {
                    mListener.onContentUri(inputContentInfo.getContentUri());
                } finally {
                    inputContentInfo.releasePermission();
                }
                return true;
            }
        };
    }
}
