package sources

import (
	"bytes"
	"context"
	_ "embed"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_getChecksum(t *testing.T) {
	type args struct {
		fname   string
		hashLen int
		r       io.Reader
	}

	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			"openwrt-x86-64-rootfs.tar.gz",
			args{
				"openwrt-x86-64-rootfs.tar.gz",
				64,
				bytes.NewBufferString(`8b194c619b65d675da15d190fe7c7d2ce6125debc98452e30890c16212aa7b1c *openwrt-imagebuilder-x86-64.Linux-x86_64.tar.xz
d99669ef301129e6ba59417ff41814dd02b4bdbe7254e2c8535de5eae35801ad *openwrt-sdk-x86-64_gcc-8.4.0_musl.Linux-x86_64.tar.xz
84be5c09beb3791c574a35b9e73dcb7b7637482f83ed61fbe07cd0af68987cf8 *openwrt-x86-64-generic-ext4-combined-efi.img.gz
23d9ac551d0cd9c85458d4032ae030f33f5f6b44158866130c3065f2a121b641 *openwrt-x86-64-generic-ext4-combined.img.gz
4462e51e9b325e107b57a3b44aef176837fcee0ae8ccc01c1e239e343c9666e0 *openwrt-x86-64-generic-ext4-rootfs.img.gz
643ff73b119f3ecb36497a0c71213f9dd0129b64e803fa87d7e75b39c730e7fa *openwrt-x86-64-generic-kernel.bin
770fa5a3e47ed12f46114aca6dca16a1a4ba2b6e89e53d5966839ffc5581dc53 *openwrt-x86-64-generic-squashfs-combined-efi.img.gz
1a19c82c0614ad043fa0b854249bf6cc804550359ec453816ffbd426c31ab4a2 *openwrt-x86-64-generic-squashfs-combined.img.gz
3b961a97e3105e02e07c1aba7671186efe559ce0ac078c370d5082a7a6999dbe *openwrt-x86-64-generic-squashfs-rootfs.img.gz
76cc26429a61a516d348735a8d62bf3885d9d37489f20789a77c879dcf8a1025 *openwrt-x86-64-rootfs.tar.gz`),
			},
			[]string{"76cc26429a61a516d348735a8d62bf3885d9d37489f20789a77c879dcf8a1025"},
		},
		{
			"stage3-ppc64le-20200414T103003Z.tar.xz",
			args{
				"stage3-ppc64le-20200414T103003Z.tar.xz",
				128,
				bytes.NewBufferString(`# BLAKE2 (b2sum) HASH
2c5dc7ce04e4d72204a513e4bfa4bd0129e61a060747537ca748538ea8ed6016656f84c35b4cf2049df91a164977d1d0e506e722443fdb48874e9a0b90c00f7a  /var/tmp/catalyst/builds/default/stage3-ppc64le-20200414T103003Z.tar.xz
# SHA512 HASH
e4b9cb10146502310cbedf14197afa9e94b75f7d59c1c6977bff23bac529e9114e3fddb155cfcad9119e466a39f0fcd8d75354e5237da79c9289fe76ee77693d  stage3-ppc64le-20200414T103003Z.tar.xz
# BLAKE2 (b2sum) HASH
7e1a1985a41b61ac24c4fdefe7a09237161dc7ff20150f3e02c73115b74778f96c45042ced08e38c931ad6e316dfef80ac3a4c956fcd16528819dd506a320726  /var/tmp/catalyst/builds/default/stage3-ppc64le-20200414T103003Z.tar.xz.CONTENTS
# SHA512 HASH
1047f97cbb209fb22d372dffe4461722b5eaf936fc73546a8f036dc52a5d20433921d367288b28b3de5154cad1253b40d32233104c2be45732ebfa413bd9b09b  stage3-ppc64le-20200414T103003Z.tar.xz.CONTENTS`),
			},
			[]string{"e4b9cb10146502310cbedf14197afa9e94b75f7d59c1c6977bff23bac529e9114e3fddb155cfcad9119e466a39f0fcd8d75354e5237da79c9289fe76ee77693d"},
		},
		{
			"CentOS-8-x86_64-1905-dvd1.iso",
			args{
				"CentOS-8-x86_64-1905-dvd1.iso",
				64,
				bytes.NewBufferString(`-----BEGIN PGP SIGNED MESSAGE-----
Hash: SHA256

# CentOS-8-x86_64-1905-boot.iso: 559939584 bytes
SHA256 (CentOS-8-x86_64-1905-boot.iso) = a7993a0d4b7fef2433e0d4f53530b63c715d3aadbe91f152ee5c3621139a2cbc
# CentOS-8-x86_64-1905-dvd1.iso: 7135559680 bytes
SHA256 (CentOS-8-x86_64-1905-dvd1.iso) = ea17ef71e0df3f6bf1d4bf1fc25bec1a76d1f211c115d39618fe688be34503e8
-----BEGIN PGP SIGNATURE-----
Version: GnuPG v1

iQIVAwUBXYirdQW1VbOEg8ZdAQigchAAj+LbZtV7BQTnfB3i+fzECuomjsTZE8Ki
zUs9fLA67aayBL1KiavIzURMgjqj/+dXWr73Kv49pELngrznPlEPOclCaPkAKSe0
V2Nj56AUhT/tHGcBoNvD0UrC0nCObMLx6PI2FDEozEELyQR32Syjtb0y5CDnxRvX
6JeGWPWQsf+jXdZS/GUUh39XR5va5YAwues0qLfqNf7nfUk07tmU0pcCG+vRN13H
45av+1/49zbxn4Y/Km2AaAbmqX8LlQpppVYE2K5V73YsG3o6eSU1DwjDijQHYPOK
ZUixjbhh5xkOzvhv5HUETvPncbnOez+xLwDPFAMFz/jX/4BgLWpA1/PM/3xcFFij
qXBlZh+QLWm1Z8UCBftDc+RqoktI460cqL/SsnOyHmQ+95QLt20yR46hi3oZ6/Cv
cUdXaql3iCNWZUvi27Dr8bExqaVaJn0zeDCItPWUA7NwxXP2TlGs2VXC4E37HQhZ
SyuCQZMrwGmDJl7gMOE7kZ/BifKvrycAlvTPvhq0jrUwLvokX8QhoTmAwRdzwGSk
9nS+BkoK7xW5lSATuVYEcCkb2fL+qDKuSBJMuKhQNhPs6rN5OEZL3gU54so7Jyz9
OmR+r+1+/hELjOIsPcR4IiyauJQXXgtJ28G7swMsrl07PYHOU+awvB/N9GyUzNAM
RP3G/3Z1T3c=
=HgZm
-----END PGP SIGNATURE-----`),
			},
			[]string{"ea17ef71e0df3f6bf1d4bf1fc25bec1a76d1f211c115d39618fe688be34503e8"},
		},

		{
			"CentOS-7-x86_64-Minimal-1908.iso",
			args{
				"CentOS-7-x86_64-Minimal-1908.iso",
				64,
				bytes.NewBufferString(`-----BEGIN PGP SIGNED MESSAGE-----
Hash: SHA256

9bba3da2876cb9fcf6c28fb636bcbd01832fe6d84cd7445fa58e44e569b3b4fe  CentOS-7-x86_64-DVD-1908.iso
bd5e6ca18386e8a8e0b5a9e906297b5610095e375e4d02342f07f32022b13acf  CentOS-7-x86_64-Everything-1908.iso
ba827210d4eb9313fc19120b9b85e7baef234c7f81bc55847a336114ddac20cb  CentOS-7-x86_64-LiveGNOME-1908.iso
0ef3310d13f7fc140ec5180dc05369d2f473e802577466825205d17e46ef5a9b  CentOS-7-x86_64-LiveKDE-1908.iso
9a2c47d97b9975452f7d582264e9fc16d108ed8252ac6816239a3b58cef5c53d  CentOS-7-x86_64-Minimal-1908.iso
6ffa7ad44e8716e4cd6a5c3a85ba5675a935fc0448c260f43b12311356ba85ad  CentOS-7-x86_64-NetInstall-1908.iso
-----BEGIN PGP SIGNATURE-----
Version: GnuPG v1

iQIVAwUBXYDRPyTGqKf0qA61AQhHcg/+LvGu95Y825HoUpS9JPFIb7axkIj8fx5/
Qw2fN+BQtd7W7jcUNmofaajjWyqP5b5Q0iCyNrbhAT6CO4lVVY1z+OxCefAk/Wve
go1fSY5cRn7LRtvDuKrkDHJE+nYCVBg8ksWRBm2Xwx2sy4AxP2PAs7Oh3QvkK+9V
199YPLAQ+m4cFdBTTR3Dl78OEKVgjp5O351n4q0pKp72jxhjCZ+tk+dWGg9JEBSb
53nMkwnqTWZzFYpLqGc3fOfscc38oIvet0y3gVbZLNsE25AwwMxqjlC/Z2TqXwc5
1JoZI7XkKggWH6fA4BuzcOtezGMPMPDaqnNhfAWzYq3CsQAA8aQuQaCnGoG2dNN/
fdhGRrbXdpAFbKhfQ/dbKSvDGNvZTFfRfD9m5AJ/ddUAv7DFr4VeVur1KMTqtVO2
NvcLRn7BnkN7ZRqvqdT4kDyndWgQCABahqI6OcC8mmc449JecloQK4U1zGhKMRor
33OtMEW/KhnSOu9pK6+CRnPykyIk2yxUCJ11YFXCKNKfX2cmdFf0puUsmefB6O7E
1nVE3n0aZVSVmebl3sjVJvstT2oyVNynnSQ/Fw3NBAiHe5FvgUnVqHQKyg1nnTet
hsfTg6egTQUGOB2fVgt7n3p1HIvCjXAjKo6Wa3R8+aoapQ74Gcok3I3rNoL1jWbW
Z4iksZrx82g=
=L746
-----END PGP SIGNATURE-----`),
			},
			[]string{"9a2c47d97b9975452f7d582264e9fc16d108ed8252ac6816239a3b58cef5c53d"},
		},
		{
			"stage3-ppc-20200518T100528Z.tar.xz",
			args{
				"stage3-ppc-20200518T100528Z.tar.xz",
				128,
				bytes.NewBufferString(`# BLAKE2B HASH
				6298bdc913c83190f6aa5f7399f05f2f1a20b1997479f033a261fa2e8347fd7cee67900a761c47b7c1c8370127e58016dd90d58f2f37b7f0d5e16722ba0650d2  stage3-ppc-20200518T100528Z.tar.xz
				# SHA512 HASH
				2d0183b8151e4560c317c2903c330f9fbfe2cc37c02ee100a60c9f42253e3ac3ef6db341c177a5e7594131bdcbdebfabe73217c0d4dc86e4dc4d1ce59ad5fbe7  stage3-ppc-20200518T100528Z.tar.xz
				# BLAKE2B HASH
				f8aeda7504be4a1374cbd837703138880baf70f8b256ee9f1f2f90cea0b669de62b14112afd2302ff03b6b410cd84f7434a79af3cb197c896a8279ca3068cdfe  stage3-ppc-20200518T100528Z.tar.xz.CONTENTS.gz
				# SHA512 HASH
				3a7dede7bcb68a0a32310d1bfbdd8806a17a1720be30907a17673f5f303dee340f5ad9c99d25738fb6f65b5ec224786b7d6b3ecbd5f37185469fbf33ea4c8c92  stage3-ppc-20200518T100528Z.tar.xz.CONTENTS.gz`),
			},
			[]string{
				"6298bdc913c83190f6aa5f7399f05f2f1a20b1997479f033a261fa2e8347fd7cee67900a761c47b7c1c8370127e58016dd90d58f2f37b7f0d5e16722ba0650d2",
				"2d0183b8151e4560c317c2903c330f9fbfe2cc37c02ee100a60c9f42253e3ac3ef6db341c177a5e7594131bdcbdebfabe73217c0d4dc86e4dc4d1ce59ad5fbe7",
			},
		},
	}

	for _, tt := range tests {
		got := getChecksum(tt.args.fname, tt.args.hashLen, tt.args.r)
		require.Equal(t, tt.want, got)
	}
}

var (
	//go:embed "testdata/key1.pub"
	testdataKey1 string
	//go:embed "testdata/key2.pub"
	testdataKey2 string
	//go:embed "testdata/key3.pub"
	testdataKey3 string
	//go:embed "testdata/key4.pub"
	testdataKey4 string
	//go:embed "testdata/key5.pub"
	testdataKey5 string
)

func TestShowFingerprint(t *testing.T) {
	tcs := []struct {
		publicKey string
		want      string
	}{
		{testdataKey1, "A1BD8E9D78F7FE5C3E65D8AF8B48AD6246925553"},
		{testdataKey2, "F6ECB3762474EDA9D21B7022871920D1991BC93C"},
		{testdataKey3, "790BC7277767219C42C86F933B4FE6ACC0B21F32"},
		{testdataKey4, "CF24B9C038097D8A44958E2C8DEBDA68B48282A4"},
		{testdataKey5, "C1DAC52D1664E8A4386DBA430946FCA2C105B9DE"},
		{"invalid public key", ""},
	}

	for _, tc := range tcs {
		t.Run("", func(t *testing.T) {
			gpgDir, err := os.MkdirTemp("", "gpg")
			if err != nil {
				t.Fatal(err)
			}

			defer func() {
				os.RemoveAll(gpgDir)
			}()
			fingerprint, err := showFingerprint(context.Background(), gpgDir, tc.publicKey)
			if tc.want == "" {
				if err == nil {
					t.Fatal(err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}

				if fingerprint != tc.want {
					t.Fatal(fingerprint, tc.want)
				}
			}
		})
	}
}

func TestImportPublicKeys(t *testing.T) {
	tcs := []struct {
		publicKeys []string
		want       string
	}{
		{[]string{testdataKey1, testdataKey2, testdataKey3, testdataKey4, testdataKey5}, "OK"},
		{[]string{testdataKey1, testdataKey2, testdataKey3, testdataKey4, testdataKey5, testdataKey1}, "OK"},
		{[]string{testdataKey1, testdataKey2, testdataKey3, testdataKey4, testdataKey5, "invalid public key"}, "NotOK"},
	}

	for _, tc := range tcs {
		t.Run("", func(t *testing.T) {
			gpgDir, err := os.MkdirTemp("", "gpg")
			if err != nil {
				t.Fatal(err)
			}

			defer func() {
				os.RemoveAll(gpgDir)
			}()
			err = importPublicKeys(context.Background(), gpgDir, tc.publicKeys)
			if tc.want == "OK" {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil {
					t.Fatal()
				}
			}
		})
	}
}

func TestRecvGPGKeys(t *testing.T) {
	tcs := []struct {
		keys []string
		want bool
	}{
		{[]string{testdataKey1, testdataKey2, testdataKey3, testdataKey4, testdataKey5}, true},
		{[]string{testdataKey1, testdataKey2, testdataKey3, testdataKey4, testdataKey5,
			`-----BEGIN PGP PUBLIC KEY BLOCK-----
invalid public key`}, false},
	}

	for _, tc := range tcs {
		t.Run("", func(t *testing.T) {
			gpgDir, err := os.MkdirTemp("", "gpg")
			if err != nil {
				t.Fatal(err)
			}

			want, err := recvGPGKeys(context.Background(), gpgDir, "", tc.keys)
			if want != tc.want {
				t.Fatal(want, tc.want, err)
			}

			if tc.want {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil {
					t.Fatal()
				}
			}
		})
	}
}
