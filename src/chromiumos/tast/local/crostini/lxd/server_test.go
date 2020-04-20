// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lxd

import (
	"encoding/json"
	"reflect"
	"testing"
)

const imagesJSON = `{
	"content_id": "images",
	"products": {
	  "debian:buster:arm64:test": {
		"arch": "arm64",
		"versions": {
		  "20200304_22:10": {
			"items": {
			  "rootfs.tar.xz": {
				"path": "images/debian/buster/arm64/test/20200304_22:10/rootfs.tar.xz",
				"size": 330191788,
				"sha256": "c933df779c97c42a2d0df16b017590620ad620bfacc9198d1da03fc9928ced0e",
				"ftype": "root.tar.xz"
			  },
			  "lxd.tar.xz": {
				"size": 560,
				"combined_sha256": "c2ac6e679ce4382d171cce0ccf8dd92da549049453aa83329874a86cd4e33cef",
				"combined_squashfs_sha256": "d2c572ffd90dad3f755f761eb113d1e501ac2452993894d39f3f7a88ba80c6f3",
				"path": "images/debian/buster/arm64/test/20200304_22:10/lxd.tar.xz",
				"sha256": "7b82e33f573eab4c969b8c3b40a15ad32b654a3e02a71f6495e315662d91f5a1",
				"combined_rootxz_sha256": "c2ac6e679ce4382d171cce0ccf8dd92da549049453aa83329874a86cd4e33cef",
				"ftype": "lxd.tar.xz"
			  },
			  "rootfs.squashfs": {
				"path": "images/debian/buster/arm64/test/20200304_22:10/rootfs.squashfs",
				"size": 457416704,
				"sha256": "501537ee90082e9789773cece03714bc171c810529b1d2c0aa71c26ff0b81c72",
				"ftype": "squashfs"
			  }
			}
		  }
		},
		"release": "buster",
		"os": "Debian",
		"release_title": "buster",
		"aliases": "debian/buster/test,debian/buster/test/arm64"
	  },
	  "debian:stretch:amd64:default": {
		"arch": "amd64",
		"versions": {
		  "20200304_22:10": {
			"items": {
			  "rootfs.tar.xz": {
				"path": "images/debian/stretch/amd64/default/20200304_22:10/rootfs.tar.xz",
				"size": 257957064,
				"sha256": "b74a9f22f0c2e81767687936bb96cf59373d1fab68035ef5a36010dde897df2e",
				"ftype": "root.tar.xz"
			  },
			  "lxd.tar.xz": {
				"size": 560,
				"combined_sha256": "ca8bd01717304e6f4ce3bb73d11a4b0127c5f0dafe2104c108f6b7254b3ff160",
				"combined_squashfs_sha256": "6b96347da67db6fa1a5229c582d3dfc592160392ba322648fc2e3a57fe981129",
				"path": "images/debian/stretch/amd64/default/20200304_22:10/lxd.tar.xz",
				"sha256": "5c4712bce814192f8053a1b91cdc9cc18e5baeb60765426e680c14b82c2e6447",
				"combined_rootxz_sha256": "ca8bd01717304e6f4ce3bb73d11a4b0127c5f0dafe2104c108f6b7254b3ff160",
				"ftype": "lxd.tar.xz"
			  },
			  "rootfs.squashfs": {
				"path": "images/debian/stretch/amd64/default/20200304_22:10/rootfs.squashfs",
				"size": 341495808,
				"sha256": "068f8211465647bcdeda3ae6221bc9f88126e5534fbd430a8989211b48508233",
				"ftype": "squashfs"
			  }
			}
		  }
		},
		"release": "stretch",
		"os": "Debian",
		"release_title": "stretch",
		"aliases": "debian/stretch/default,debian/stretch/default/amd64,debian/stretch,debian/stretch/amd64"
	  },
	  "debian:buster:amd64:default": {
		"arch": "amd64",
		"versions": {
		  "20200304_22:10": {
			"items": {
			  "rootfs.tar.xz": {
				"path": "images/debian/buster/amd64/default/20200304_22:10/rootfs.tar.xz",
				"size": 258840276,
				"sha256": "b860abfe6385289ead913bdb0e1cb8a3df92be8e5636bed00ace175258a51b6a",
				"ftype": "root.tar.xz"
			  },
			  "lxd.tar.xz": {
				"size": 560,
				"combined_sha256": "a777b01321f0781cd2d8d17ec5d487efb316ed012c24657d57900a2d9ffd9fd9",
				"combined_squashfs_sha256": "deae1ebaa83fa743ba9c33e942f045f45fd22430c15eb22b65af71a00b106005",
				"path": "images/debian/buster/amd64/default/20200304_22:10/lxd.tar.xz",
				"sha256": "d96e64e86fe314b41717a270e394fc644bddf23ebbdee870a8dec034435a47af",
				"combined_rootxz_sha256": "a777b01321f0781cd2d8d17ec5d487efb316ed012c24657d57900a2d9ffd9fd9",
				"ftype": "lxd.tar.xz"
			  },
			  "rootfs.squashfs": {
				"path": "images/debian/buster/amd64/default/20200304_22:10/rootfs.squashfs",
				"size": 361897984,
				"sha256": "da076d1d6b0944a68ecdb86ff3711999f1a2d23645b3a457e99b02908a77852b",
				"ftype": "squashfs"
			  }
			}
		  }
		},
		"release": "buster",
		"os": "Debian",
		"release_title": "buster",
		"aliases": "debian/buster/default,debian/buster/default/amd64,debian/buster,debian/buster/amd64"
	  },
	  "debian:stretch:amd64:test": {
		"arch": "amd64",
		"versions": {
		  "20200304_22:10": {
			"items": {
			  "rootfs.tar.xz": {
				"path": "images/debian/stretch/amd64/test/20200304_22:10/rootfs.tar.xz",
				"size": 367523204,
				"sha256": "9ba2b64ebb340e33aebf1b21fbd11a085ddb7624f41ecc88a0d212c5570d3c09",
				"ftype": "root.tar.xz"
			  },
			  "lxd.tar.xz": {
				"size": 560,
				"combined_sha256": "4b3dc2641d3468b0d108ed23952432d66d3156e41fa9ee33b89c8e48b741f077",
				"combined_squashfs_sha256": "4a26115a7539e3bfd5160f4f0016ecf1fe429e07ef3a456f88ae5e886a3b2b3a",
				"path": "images/debian/stretch/amd64/test/20200304_22:10/lxd.tar.xz",
				"sha256": "5c4712bce814192f8053a1b91cdc9cc18e5baeb60765426e680c14b82c2e6447",
				"combined_rootxz_sha256": "4b3dc2641d3468b0d108ed23952432d66d3156e41fa9ee33b89c8e48b741f077",
				"ftype": "lxd.tar.xz"
			  },
			  "rootfs.squashfs": {
				"path": "images/debian/stretch/amd64/test/20200304_22:10/rootfs.squashfs",
				"size": 469774336,
				"sha256": "c1f1dc8d143ff99aa1b5c8dcdcacf81e4a4916e761e12de8f3de69293478758e",
				"ftype": "squashfs"
			  }
			}
		  }
		},
		"release": "stretch",
		"os": "Debian",
		"release_title": "stretch",
		"aliases": "debian/stretch/test,debian/stretch/test/amd64"
	  },
	  "debian:stretch:arm64:test": {
		"arch": "arm64",
		"versions": {
		  "20200304_22:10": {
			"items": {
			  "rootfs.tar.xz": {
				"path": "images/debian/stretch/arm64/test/20200304_22:10/rootfs.tar.xz",
				"size": 328022624,
				"sha256": "caff1336fd0683aac66cc1ee0c7efd637cf707e13c0945ddde1456f4b0d6e153",
				"ftype": "root.tar.xz"
			  },
			  "lxd.tar.xz": {
				"size": 564,
				"combined_sha256": "b42cb60fdca60e208cdaad189c5d64d95f80c2651c5ba5d6e1c7d734a74d9f2d",
				"combined_squashfs_sha256": "5b8277dfd638e000942aea21148c3fad71f7c1a48eee5c3b6f3298f434c4ea9c",
				"path": "images/debian/stretch/arm64/test/20200304_22:10/lxd.tar.xz",
				"sha256": "fb8a29887a4bf171fbf7c6a48f1cf9a89a0e56e54e51fd8fecdb8822b89b1d15",
				"combined_rootxz_sha256": "b42cb60fdca60e208cdaad189c5d64d95f80c2651c5ba5d6e1c7d734a74d9f2d",
				"ftype": "lxd.tar.xz"
			  },
			  "rootfs.squashfs": {
				"path": "images/debian/stretch/arm64/test/20200304_22:10/rootfs.squashfs",
				"size": 426278912,
				"sha256": "cb692361502c1d30ea56a5179d1bf6f952fe3595b8c1938d10f4efc9ce96903b",
				"ftype": "squashfs"
			  }
			}
		  }
		},
		"release": "stretch",
		"os": "Debian",
		"release_title": "stretch",
		"aliases": "debian/stretch/test,debian/stretch/test/arm64"
	  },
	  "debian:buster:arm64:default": {
		"arch": "arm64",
		"versions": {
		  "20200304_22:10": {
			"items": {
			  "rootfs.tar.xz": {
				"path": "images/debian/buster/arm64/default/20200304_22:10/rootfs.tar.xz",
				"size": 242546960,
				"sha256": "ded6a6af82d23b276699958c5a97176aa615657c10ec55e28aecce8e16fad494",
				"ftype": "root.tar.xz"
			  },
			  "lxd.tar.xz": {
				"size": 560,
				"combined_sha256": "61b5b7d80d194ee0b9770a603965970fe1101a610b1561b303329bec166a3126",
				"combined_squashfs_sha256": "1053a5edc9897b7461c0c080bdfc0948d33fd134339773a4d0eaf29affdfdd6e",
				"path": "images/debian/buster/arm64/default/20200304_22:10/lxd.tar.xz",
				"sha256": "7b82e33f573eab4c969b8c3b40a15ad32b654a3e02a71f6495e315662d91f5a1",
				"combined_rootxz_sha256": "61b5b7d80d194ee0b9770a603965970fe1101a610b1561b303329bec166a3126",
				"ftype": "lxd.tar.xz"
			  },
			  "rootfs.squashfs": {
				"path": "images/debian/buster/arm64/default/20200304_22:10/rootfs.squashfs",
				"size": 348250112,
				"sha256": "27c743be3d950646b976d7018bcca3ca51e0a871eeeed72ceb6235bfaf9da3b4",
				"ftype": "squashfs"
			  }
			}
		  }
		},
		"release": "buster",
		"os": "Debian",
		"release_title": "buster",
		"aliases": "debian/buster/default,debian/buster/default/arm64,debian/buster,debian/buster/arm64"
	  },
	  "debian:stretch:arm64:default": {
		"arch": "arm64",
		"versions": {
		  "20200304_22:10": {
			"items": {
			  "rootfs.tar.xz": {
				"path": "images/debian/stretch/arm64/default/20200304_22:10/rootfs.tar.xz",
				"size": 232776876,
				"sha256": "579112a9223d2f801311f1c001782629ede6aaf265204ba344e24ff1fd57314b",
				"ftype": "root.tar.xz"
			  },
			  "lxd.tar.xz": {
				"size": 564,
				"combined_sha256": "0be1e40abafc1a91aa03a20952f36769c1645843c337403f1e21058c6365ee9c",
				"combined_squashfs_sha256": "8a16efc751ef4487b5d9c5621f50383802b5cabf2e92efd4938ac9320cef2af2",
				"path": "images/debian/stretch/arm64/default/20200304_22:10/lxd.tar.xz",
				"sha256": "fb8a29887a4bf171fbf7c6a48f1cf9a89a0e56e54e51fd8fecdb8822b89b1d15",
				"combined_rootxz_sha256": "0be1e40abafc1a91aa03a20952f36769c1645843c337403f1e21058c6365ee9c",
				"ftype": "lxd.tar.xz"
			  },
			  "rootfs.squashfs": {
				"path": "images/debian/stretch/arm64/default/20200304_22:10/rootfs.squashfs",
				"size": 314417152,
				"sha256": "b7cdbee5006d7851eb6d5cf2ce65be99c80dfafb639a8a249580a7826a6cbd48",
				"ftype": "squashfs"
			  }
			}
		  }
		},
		"release": "stretch",
		"os": "Debian",
		"release_title": "stretch",
		"aliases": "debian/stretch/default,debian/stretch/default/arm64,debian/stretch,debian/stretch/arm64"
	  },
	  "debian:buster:amd64:test": {
		"arch": "amd64",
		"versions": {
		  "20200304_22:10": {
			"items": {
			  "rootfs.tar.xz": {
				"path": "images/debian/buster/amd64/test/20200304_22:10/rootfs.tar.xz",
				"size": 351730528,
				"sha256": "d8665ebc8bdea4350d643de10cead5b809222be8e5575f219ec0f0a2e90758b3",
				"ftype": "root.tar.xz"
			  },
			  "lxd.tar.xz": {
				"size": 560,
				"combined_sha256": "b080a08457cb6090f67f160d6d1823b4c4226b8fcb92cea6f727995b9af36f84",
				"combined_squashfs_sha256": "1f5281f52f8f373ea35e6523d9ac2af6091ee81299a6fa3cd5695c63f2f0b187",
				"path": "images/debian/buster/amd64/test/20200304_22:10/lxd.tar.xz",
				"sha256": "d96e64e86fe314b41717a270e394fc644bddf23ebbdee870a8dec034435a47af",
				"combined_rootxz_sha256": "b080a08457cb6090f67f160d6d1823b4c4226b8fcb92cea6f727995b9af36f84",
				"ftype": "lxd.tar.xz"
			  },
			  "rootfs.squashfs": {
				"path": "images/debian/buster/amd64/test/20200304_22:10/rootfs.squashfs",
				"size": 477302784,
				"sha256": "068c0b67753594071f38d958f9c9c3b07f1b76d5ba42f90438da8927c7817b17",
				"ftype": "squashfs"
			  }
			}
		  }
		},
		"release": "buster",
		"os": "Debian",
		"release_title": "buster",
		"aliases": "debian/buster/test,debian/buster/test/amd64"
	  }
	},
	"datatype": "image-downloads",
	"format": "products:1.0"
  }`

const reducedImagesJSON = `{
	"content_id": "images",
	"products": {
	  "debian:buster:amd64:test": {
		"arch": "amd64",
		"versions": {
		  "20200304_22:10": {
			"items": {
			  "rootfs.tar.xz": {
				"path": "images/debian/buster/amd64/test/20200304_22:10/rootfs.tar.xz",
				"size": 351730528,
				"sha256": "d8665ebc8bdea4350d643de10cead5b809222be8e5575f219ec0f0a2e90758b3",
				"ftype": "root.tar.xz"
			  },
			  "lxd.tar.xz": {
				"size": 560,
				"combined_sha256": "b080a08457cb6090f67f160d6d1823b4c4226b8fcb92cea6f727995b9af36f84",
				"combined_squashfs_sha256": "1f5281f52f8f373ea35e6523d9ac2af6091ee81299a6fa3cd5695c63f2f0b187",
				"path": "images/debian/buster/amd64/test/20200304_22:10/lxd.tar.xz",
				"sha256": "d96e64e86fe314b41717a270e394fc644bddf23ebbdee870a8dec034435a47af",
				"combined_rootxz_sha256": "b080a08457cb6090f67f160d6d1823b4c4226b8fcb92cea6f727995b9af36f84",
				"ftype": "lxd.tar.xz"
			  },
			  "rootfs.squashfs": {
				"path": "images/debian/buster/amd64/test/20200304_22:10/rootfs.squashfs",
				"size": 477302784,
				"sha256": "068c0b67753594071f38d958f9c9c3b07f1b76d5ba42f90438da8927c7817b17",
				"ftype": "squashfs"
			  }
			}
		  }
		},
		"release": "buster",
		"os": "Debian",
		"release_title": "buster",
		"aliases": "debian/buster/test,debian/buster/test/amd64"
	  }
	},
	"datatype": "image-downloads",
	"format": "products:1.0"
  }`

func containsString(paths []string, path string) bool {
	for _, p := range paths {
		if p == path {
			return true
		}
	}
	return false
}

func TestReduceImage(t *testing.T) {
	actualJSON, paths, err := ReduceImagesJSON([]byte(imagesJSON), "debian:buster:amd64:test")
	if err != nil {
		t.Errorf("error from reducsImagesJSON(): got %v, want nil", err)
	}
	var actual map[string]interface{}
	var expected map[string]interface{}
	if err := json.Unmarshal(actualJSON, &actual); err != nil {
		t.Errorf("Failed to unmarshal actual value: %v", err)
	}
	if err := json.Unmarshal([]byte(reducedImagesJSON), &expected); err != nil {
		t.Errorf("Failed to unmarshal actual value: %v", err)
	}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("JSON doesn't match.  got:\n%v\n\nwant:%v\n\n", string(actualJSON), reducedImagesJSON)
	}
	if len(paths) != 3 {
		t.Errorf("Wrong number of paths returned: got %d, want 3", len(paths))
	}
	expectedPaths := []string{
		"images/debian/buster/amd64/test/20200304_22:10/lxd.tar.xz",
		"images/debian/buster/amd64/test/20200304_22:10/rootfs.squashfs",
		"images/debian/buster/amd64/test/20200304_22:10/rootfs.tar.xz",
	}
	for _, expected := range expectedPaths {
		if !containsString(paths, expected) {
			t.Errorf("Missing expected path, got [%v], want %s", paths, expected)
		}
	}
}
