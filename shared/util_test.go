package shared

import (
	"bytes"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"testing"

	lxd "github.com/lxc/lxd/shared"
	"github.com/stretchr/testify/require"
	"gopkg.in/flosch/pongo2.v3"
)

func TestVerifyFile(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to retrieve working directory: %v", err)
	}
	testdataDir := filepath.Join(wd, "..", "testdata")

	keys := []string{"0x5DE8949A899C8D99"}
	keyserver := "keyserver.ubuntu.com"

	tests := []struct {
		name          string
		signedFile    string
		signatureFile string
		keys          []string
		keyserver     string
		shouldFail    bool
	}{
		{
			"testfile with detached signature",
			filepath.Join(testdataDir, "testfile"),
			filepath.Join(testdataDir, "testfile.sig"),
			keys,
			keyserver,
			false,
		},
		{
			"testfile with cleartext signature",
			filepath.Join(testdataDir, "testfile.asc"),
			"",
			keys,
			keyserver,
			false,
		},
		{
			"testfile with invalid cleartext signature",
			filepath.Join(testdataDir, "testfile-invalid.asc"),
			"",
			keys,
			keyserver,
			true,
		},
		{
			"testfile with normal signature",
			filepath.Join(testdataDir, "testfile.gpg"),
			"",
			keys,
			keyserver,
			false,
		},
		{
			"no keys",
			filepath.Join(testdataDir, "testfile"),
			filepath.Join(testdataDir, "testfile.sig"),
			[]string{},
			keyserver,
			true,
		},
		{
			"invalid key",
			filepath.Join(testdataDir, "testfile.asc"),
			"",
			[]string{"0x46181433FBB75451"},
			keyserver,
			true,
		},
	}

	for i, tt := range tests {
		log.Printf("Running test #%d: %s", i, tt.name)
		valid, err := VerifyFile(tt.signedFile, tt.signatureFile, tt.keys,
			tt.keyserver)
		if tt.shouldFail {
			require.Error(t, err)
			require.False(t, valid)
		} else {
			require.NoError(t, err)
			require.True(t, valid)
		}
	}
}

func TestCreateGPGKeyring(t *testing.T) {
	keyring, err := CreateGPGKeyring("keyserver.ubuntu.com", []string{"0x5DE8949A899C8D99"})
	require.NoError(t, err)

	require.FileExists(t, keyring)
	os.RemoveAll(path.Dir(keyring))

	// This shouldn't fail, but the keyring file should not be created since
	// there are no keys to be exported.
	keyring, err = CreateGPGKeyring("", []string{})
	require.NoError(t, err)

	require.False(t, lxd.PathExists(keyring), "File should not exist")
	os.RemoveAll(path.Dir(keyring))
}

func TestRenderTemplate(t *testing.T) {
	tests := []struct {
		name       string
		iface      interface{}
		template   string
		expected   string
		shouldFail bool
	}{
		{
			"valid template with yaml tags",
			Definition{
				Image: DefinitionImage{
					Distribution: "Ubuntu",
					Release:      "Bionic",
				},
			},
			"{{ image.distribution }} {{ image.release }}",
			"Ubuntu Bionic",
			false,
		},
		{
			"valid template without yaml tags",
			pongo2.Context{
				"foo": "bar",
			},
			"{{ foo }}",
			"bar",
			false,
		},
		{
			"variable not in context",
			pongo2.Context{},
			"{{ foo }}",
			"",
			false,
		},
		{
			"invalid template",
			pongo2.Context{
				"foo": nil,
			},
			"{{ foo }",
			"",
			true,
		},
		{
			"invalid context",
			pongo2.Context{
				"foo.bar": nil,
			},
			"{{ foo.bar }}",
			"",
			true,
		},
	}

	for i, tt := range tests {
		log.Printf("Running test #%d: %s", i, tt.name)
		ret, err := RenderTemplate(tt.template, tt.iface)
		if tt.shouldFail {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, tt.expected, ret)
		}
	}
}

func TestSetEnvVariables(t *testing.T) {
	// Initial variables
	os.Setenv("FOO", "bar")

	env := Environment{
		"FOO": EnvVariable{
			Value: "bla",
			Set:   true,
		},
		"BAR": EnvVariable{
			Value: "blub",
			Set:   true,
		},
	}

	// Set new env variables
	oldEnv := SetEnvVariables(env)

	for k, v := range env {
		val, set := os.LookupEnv(k)
		require.True(t, set)
		require.Equal(t, v.Value, val)
	}

	// Reset env variables
	SetEnvVariables(oldEnv)

	val, set := os.LookupEnv("FOO")
	require.True(t, set)
	require.Equal(t, val, "bar")

	val, set = os.LookupEnv("BAR")
	require.False(t, set, "Expected 'BAR' to be unset")
	require.Empty(t, val)
}

func Test_getChecksum(t *testing.T) {
	type args struct {
		fname   string
		hashLen int
		r       io.Reader
	}
	tests := []struct {
		name string
		args args
		want string
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
			"76cc26429a61a516d348735a8d62bf3885d9d37489f20789a77c879dcf8a1025",
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
			"e4b9cb10146502310cbedf14197afa9e94b75f7d59c1c6977bff23bac529e9114e3fddb155cfcad9119e466a39f0fcd8d75354e5237da79c9289fe76ee77693d",
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
			"ea17ef71e0df3f6bf1d4bf1fc25bec1a76d1f211c115d39618fe688be34503e8",
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
			"9a2c47d97b9975452f7d582264e9fc16d108ed8252ac6816239a3b58cef5c53d",
		},
	}

	for _, tt := range tests {
		got := getChecksum(tt.args.fname, tt.args.hashLen, tt.args.r)
		require.Equal(t, tt.want, got)
	}
}
