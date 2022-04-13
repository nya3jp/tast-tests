Bitstream file name: TSCL_A_VIDYO_4.bit
Conformance point: HM-10.0
Bitstream feature:
The bitstream is for testing temporal scalability.
The bitstream has 4 temporal layers, and was encoded with the following options as the prediction structure.

IntraPeriod                   : -1          # Period of I-Frame ( -1 = only first)
DecodingRefreshType           : 0           # Random Accesss 0:none, 1:CDR, 2:IDR
GOPSize                       : 4           # GOP Size (number of B slice = GOPSize-1)
#        Type POC QPoffset QPfactor tcOffsetDiv2 betaOffsetDiv2  temporal_id #ref_pics_active #ref_pics reference pictures predict deltaRPS #ref_idcs reference idcs
Frame1:  P    1   3        0.4624   0            0               2           4                4         -1 -5 -9 -13       0
Frame2:  P    2   2        0.4624   0            0               1           4                4         -2 -6 -10 -14      0
Frame3:  P    3   3        0.4624   0            0               2           4                4         -1 -3 -7 -11       0
Frame4:  P    4   1        0.578    0            0               0           3                3         -4 -8 -12      	   0

picture size: 416x240 (BasketballPass_416x240_50.yuv)
frame rate: 50
number of encoded frames: 73

revision 2:
	fixes the illegal nal_unit_type in some of the reference frames
	
revision 3:
	fixes the illegal nal_unit_type in leading pictures for CRA pictures

revision 4:
	added proper profile and level