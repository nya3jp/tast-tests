
LS_B_Orange_4:

conformance: HM-10.0
category: Other Coding Tools
sub category: Lossless

resolution: 832x480
# of frames: 25
frame rate: 30fps

Specification:
- a bitstream where at least 50% of the CUs do not use transform/quantization/filtering bypass, and where there at are least 100 CUs in each of the following categories
·	The CU is 64x64, it has cu_transquant_bypass_flag on, at least one of the neighboring CU uses SAO, at least one of the neighboring CU uses deblocking filter
·	The CU is 32x32, it has cu_transquant_bypass_flag on, at least one of the neighboring CU uses SAO, at least one of the neighboring CU uses deblocking filter
·	The CU is 16x16, it has cu_transquant_bypass_flag on, at least one of the neighboring CU uses SAO, at least one of the neighboring CU uses deblocking filter
·	The CU is 8x8, it has cu_transquant_bypass_flag on, at least one of the neighboring CU uses SAO, at least one of the neighboring CU uses deblocking filter


Purpose:
- Check that the decoder handles lossless coding.

MD5: SEI message in the bitstream




