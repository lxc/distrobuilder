package generators

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lxc/distrobuilder/shared"
)

func TestCopyGeneratorRun(t *testing.T) {
	cacheDir := filepath.Join(os.TempDir(), "distrobuilder-test")
	rootfsDir := filepath.Join(cacheDir, "rootfs")

	setup(t, cacheDir)
	defer teardown(cacheDir)

	generator, err := Load("copy", nil, cacheDir, rootfsDir, shared.DefinitionFile{
		Source: "copy_test",
		Path:   "copy_test_dir",
	}, shared.Definition{})
	require.IsType(t, &copy{}, generator)
	require.NoError(t, err)

	defer os.RemoveAll("copy_test")

	err = os.Mkdir("copy_test", os.ModePerm)
	require.NoError(t, err)
	src1, err := os.Create(filepath.Join("copy_test", "src1"))
	require.NoError(t, err)
	defer src1.Close()
	_, err = src1.WriteString("src1\n")
	require.NoError(t, err)
	src2, err := os.Create(filepath.Join("copy_test", "src2"))
	require.NoError(t, err)
	defer src2.Close()
	_, err = src2.WriteString("src2\n")
	require.NoError(t, err)
	err = os.Symlink("src1", filepath.Join("copy_test", "srcLink"))
	require.NoError(t, err)

	// <src> is a directory -> contents copied
	err = generator.Run()
	require.NoError(t, err)

	require.DirExists(t, filepath.Join(rootfsDir, "copy_test_dir"))
	require.FileExists(t, filepath.Join(rootfsDir, "copy_test_dir", "src1"))
	require.FileExists(t, filepath.Join(rootfsDir, "copy_test_dir", "src2"))
	require.FileExists(t, filepath.Join(rootfsDir, "copy_test_dir", "srcLink"))

	var destBuffer, srcBuffer bytes.Buffer
	dest, err := os.Open(filepath.Join(rootfsDir, "copy_test_dir", "src1"))
	require.NoError(t, err)
	defer dest.Close()

	io.Copy(&destBuffer, dest)
	_, err = src1.Seek(0, 0)
	require.NoError(t, err)
	io.Copy(&srcBuffer, src1)
	require.Equal(t, destBuffer.String(), srcBuffer.String())

	dest, err = os.Open(filepath.Join(rootfsDir, "copy_test_dir", "src2"))
	require.NoError(t, err)
	defer dest.Close()

	destBuffer.Reset()
	io.Copy(&destBuffer, dest)
	_, err = src2.Seek(0, 0)
	require.NoError(t, err)
	srcBuffer.Reset()
	io.Copy(&srcBuffer, src2)
	require.Equal(t, destBuffer.String(), srcBuffer.String())

	link, err := os.Readlink(filepath.Join(rootfsDir, "copy_test_dir", "srcLink"))
	require.NoError(t, err)
	require.Equal(t, "src1", link)

	// <src> as wildcard
	_, err = src1.Seek(0, 0)
	require.NoError(t, err)
	_, err = src2.Seek(0, 0)
	require.NoError(t, err)
	generator, err = Load("copy", nil, cacheDir, rootfsDir, shared.DefinitionFile{
		Source: "copy_test/src*",
		Path:   "copy_test_wildcard",
	}, shared.Definition{})
	require.IsType(t, &copy{}, generator)
	require.NoError(t, err)

	err = generator.Run()
	require.NoError(t, err)

	require.DirExists(t, filepath.Join(rootfsDir, "copy_test_wildcard"))
	require.FileExists(t, filepath.Join(rootfsDir, "copy_test_wildcard", "src1"))
	require.FileExists(t, filepath.Join(rootfsDir, "copy_test_wildcard", "src2"))

	dest, err = os.Open(filepath.Join(rootfsDir, "copy_test_wildcard", "src1"))
	require.NoError(t, err)
	defer dest.Close()

	destBuffer.Reset()
	io.Copy(&destBuffer, dest)
	_, err = src1.Seek(0, 0)
	require.NoError(t, err)
	srcBuffer.Reset()
	io.Copy(&srcBuffer, src1)

	require.Equal(t, destBuffer.String(), srcBuffer.String())

	dest, err = os.Open(filepath.Join(rootfsDir, "copy_test_wildcard", "src2"))
	require.NoError(t, err)
	defer dest.Close()

	destBuffer.Reset()
	io.Copy(&destBuffer, dest)
	_, err = src2.Seek(0, 0)
	require.NoError(t, err)
	srcBuffer.Reset()
	io.Copy(&srcBuffer, src2)

	require.Equal(t, destBuffer.String(), srcBuffer.String())

	// <src> is a file -> file copied to <dest>
	_, err = src1.Seek(0, 0)
	require.NoError(t, err)
	generator, err = Load("copy", nil, cacheDir, rootfsDir, shared.DefinitionFile{
		Source: "copy_test/src1",
	}, shared.Definition{})
	require.IsType(t, &copy{}, generator)
	require.NoError(t, err)

	err = generator.Run()
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(rootfsDir, "copy_test", "src1"))

	dest, err = os.Open(filepath.Join(rootfsDir, "copy_test", "src1"))
	require.NoError(t, err)
	defer dest.Close()

	destBuffer.Reset()
	io.Copy(&destBuffer, dest)
	_, err = src1.Seek(0, 0)
	require.NoError(t, err)
	srcBuffer.Reset()
	io.Copy(&srcBuffer, src1)

	require.Equal(t, destBuffer.String(), srcBuffer.String())

	// <src> is a file -> file copied to <dest>/
	_, err = src1.Seek(0, 0)
	require.NoError(t, err)
	generator, err = Load("copy", nil, cacheDir, rootfsDir, shared.DefinitionFile{
		Source: "copy_test/src1",
		Path:   "/hello/world/",
	}, shared.Definition{})
	require.IsType(t, &copy{}, generator)
	require.NoError(t, err)

	err = generator.Run()
	require.NoError(t, err)

	require.DirExists(t, filepath.Join(rootfsDir, "hello", "world"))
	require.FileExists(t, filepath.Join(rootfsDir, "hello", "world", "src1"))

	dest, err = os.Open(filepath.Join(rootfsDir, "hello", "world", "src1"))
	require.NoError(t, err)
	defer dest.Close()

	destBuffer.Reset()
	io.Copy(&destBuffer, dest)
	_, err = src1.Seek(0, 0)
	require.NoError(t, err)
	srcBuffer.Reset()
	io.Copy(&srcBuffer, src1)

	require.Equal(t, destBuffer.String(), srcBuffer.String())
}
