Bitstream file name
-------------------
SCREFOFF_A_QCOM_1.hevc

Bitstream feature name
----------------------
Setting scaled reference layer offset values to non-zero values

Explanation of bitstream features
---------------------------------
This bitstream is coded using the RA configuration, with two layers; layer 0 is a direct reference layer of layer 1. Although the resolutions of the two layers are different, the scaling factor is still 1. The scaled reference layer offsets are specified accordingly.

Number of output layer sets in the bitstream
--------------------------------------------
2

Number of layers present in the bitstream
-----------------------------------------
2

Profile, tier, and level (P,T,L) for each layer in each output layer set in the bitstream
-----------------------------------------------------------------------------------------
OLS 0 - L0: (main, main, 3)
	
OLS 1 - L0: (main, main, 3)
        L1: (scalable-main, main, 3.1)

Picture size of each layer
--------------------------
L0: 400 x 400
L1: 960 x 540

Frame rate of each layer (if available)
---------------------------------------
L0: 50 fps
L1: 50 fps

SHM version number used to generate the bitstream, if appropriate
-----------------------------------------------------------------
SHM-dev branch, r1079

Number of frames
----------------
20

Contact
-------
Adarsh K. Ramasubramonian (aramasub@qti.qualcomm.com)
