/**
 * @fileoverview Description of this file.
 */
function() {
    return {
        inHighVis: null,
        shareTarget: null,
        metadata: null,
        onHighVisibilityChanged: function (inHighVisibility) {
            this.inHighVis = inHighVisibility;
            console.log(this.inHighVis);
        },
        onTransferUpdate: function (shareTarget, metadata) {
            this.shareTarget = shareTarget;
            this.metadata = metadata;
            console.log(this.shareTarget);
            console.log(this.metadata);
        },
    }
}