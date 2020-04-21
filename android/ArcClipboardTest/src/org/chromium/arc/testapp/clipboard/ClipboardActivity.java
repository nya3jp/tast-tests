/*
 * Copyright 2020 The Chromium OS Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

package org.chromium.arc.testapp.clipboard;

import android.content.ClipData;
import android.content.ClipDescription;
import android.content.ClipboardManager;
import android.content.Context;
import android.app.Activity;
import android.os.Bundle;
import android.text.Editable;
import android.text.Html;
import android.text.TextUtils;
import android.view.View;
import android.widget.Button;
import android.widget.EditText;
import android.widget.TextView;

@SuppressWarnings("UnusedParameters")
public class ClipboardActivity extends Activity {
    private EditText mEditText;
    private TextView mTextView;
    private TextView mObserverView;
    private ClipboardManager.OnPrimaryClipChangedListener mClipboardListener = null;
    private static final String CLIP_LABEL_DESCRIPTION = "label";

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_clipboard);

        mEditText = (EditText) findViewById(R.id.edit_message);
        mTextView = (TextView) findViewById(R.id.text_view);
        mObserverView = (TextView) findViewById(R.id.observer_view);

        initButtons();
    }

    private void initButtons() {
        final Button copyButton = (Button) findViewById(R.id.copy_button);
        copyButton.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View view) {
                final ClipboardManager clipboard =
                        (ClipboardManager) getSystemService(Context.CLIPBOARD_SERVICE);
                final Editable editable = mEditText.getText();
                final String trimmedHTML = Html.toHtml(editable).trim();
                final String trimmedText = editable.toString().trim();

                ClipData clip =
                        ClipData.newHtmlText(CLIP_LABEL_DESCRIPTION, trimmedText, trimmedHTML);
                clipboard.setPrimaryClip(clip);
            }
        });

        final Button pasteButton = (Button) findViewById(R.id.paste_button);
        pasteButton.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View view) {
                pasteCallback(view);
            }
        });

        final Button writeHTMLButton = (Button) findViewById(R.id.write_html_button);
        writeHTMLButton.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View view) {
                // <p dir="ltr"> is added automatically by Html.fromHtml();
                // We add it manually just in case Html.fromHtml() changes in the future and
                // breaks the test
                writeHTML(getString(R.string.test_html_1234));
            }
        });

        final Button writeTextButton = (Button) findViewById(R.id.write_text_button);
        writeTextButton.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View view) {
                writeText(getString(R.string.test_text_1234));
            }
        });

        final Button enableObserverButton = (Button) findViewById(R.id.enable_observer_button);
        enableObserverButton.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View view) {
                if (mClipboardListener == null) {
                    mClipboardListener = new ClipboardManager.OnPrimaryClipChangedListener() {
                        @Override
                        public void onPrimaryClipChanged() {
                            pasteCallback(null);
                        }
                    };
                    final ClipboardManager clipboard =
                            (ClipboardManager) getSystemService(Context.CLIPBOARD_SERVICE);
                    clipboard.addPrimaryClipChangedListener(mClipboardListener);

                    mObserverView.setText(R.string.observer_ready);
                }
                // else, already registered. skip
            }
        });

        final Button disableObserverButton = (Button) findViewById(R.id.disable_observer_button);
        disableObserverButton.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View view) {
                if (mClipboardListener != null) {
                    final ClipboardManager clipboard =
                            (ClipboardManager) getSystemService(Context.CLIPBOARD_SERVICE);
                    clipboard.removePrimaryClipChangedListener(mClipboardListener);
                    mClipboardListener = null;

                    mObserverView.setText("");
                }
                // else, already unregistered. skip
            }
        });

        final Button hasClipboardButton = (Button) findViewById(R.id.has_clipboard_button);
        hasClipboardButton.setOnClickListener(
                new View.OnClickListener() {
                    @Override
                    public void onClick(View view) {
                        final ClipboardManager clipboard =
                                (ClipboardManager) getSystemService(Context.CLIPBOARD_SERVICE);
                        final boolean hasClip = clipboard.hasPrimaryClip();
                        mObserverView.setText(getString(R.string.clip_has_clipboard, hasClip));
                    }
                });

        final Button getDescriptionButton = (Button) findViewById(R.id.get_description_button);
        getDescriptionButton.setOnClickListener(
                new View.OnClickListener() {
                    @Override
                    public void onClick(View view) {
                        final ClipboardManager clipboard =
                                (ClipboardManager) getSystemService(Context.CLIPBOARD_SERVICE);
                        final ClipDescription description = clipboard.getPrimaryClipDescription();
                        mObserverView.setText(
                                getString(R.string.clip_get_description, (description != null)));
                    }
                });
    }

    private void pasteCallback(View view) {
        final ClipboardManager clipboard =
                (ClipboardManager) getSystemService(Context.CLIPBOARD_SERVICE);
        String pasteTextData = null;
        String pasteHTMLtData = null;

        final ClipData clip = clipboard.getPrimaryClip();
        if (clip != null && clip.getItemCount() > 0) {
            final ClipData.Item item = clip.getItemAt(0);
            final CharSequence textItem = item.getText();
            if (!TextUtils.isEmpty(textItem)) {
                pasteTextData = textItem.toString();
            }
            pasteHTMLtData = item.getHtmlText();
        }

        if (!TextUtils.isEmpty(pasteHTMLtData)) {
            writeHTML(pasteHTMLtData);
        } else if (!TextUtils.isEmpty(pasteTextData)) {
            writeText(pasteTextData);
        }
    }

    private void writeText(final String text) {
        mEditText.setText(text);
        mTextView.setText(text);
    }

    private void writeHTML(final String markup) {
        mEditText.setText(Html.fromHtml(markup));
        mTextView.setText(markup);
    }
}
