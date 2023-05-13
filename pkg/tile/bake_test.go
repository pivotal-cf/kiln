package tile_test

import (
	"bytes"
	"github.com/pivotal-cf/kiln/pkg/cargo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pivotal-cf/kiln/pkg/proofing"
	"github.com/pivotal-cf/kiln/pkg/tile"
)

func TestMetadata(t *testing.T) {
	t.Run("minimal", func(t *testing.T) {
		compareLegacyMetadata(t, testdataTile(t), nil)
	})

	t.Run("references", func(t *testing.T) {
		compareLegacyMetadata(t, testdataTile(t), nil)
	})

	t.Run("tile", func(t *testing.T) {
		tileDirectory, found := os.LookupEnv("TILE_DIRECTORY")
		if !found {
			t.Skip("TILE_DIRECTORY not set")
		}
		t.Log(tileDirectory)
		bakeConfiguration := new(tile.BakeConfiguration)
		lock, err := cargo.ReadKilnfileLock(filepath.Join(tileDirectory, "Kilnfile"))
		require.NoError(t, err)
		bakeConfiguration.Releases = lock.MetadataReleases()
		require.NoError(t, bakeConfiguration.CalculateSourceChecksum(tileDirectory))
		compareLegacyMetadata(t, tileDirectory, bakeConfiguration)
	})
}

func TestBake(t *testing.T) {
	t.Run("minimal", func(t *testing.T) {
		tileDir := bakeTestTile(t, testdataTile(t))
		err := tile.Bake(nil, tileDir, nil, nil)
		require.NoError(t, err)

		runKilnTest(t)
	})
}

func TestBakeConfiguration_CalculateSourceChecksum(t *testing.T) {
	dir := t.TempDir()

	gitInit := exec.Command("git", "init")
	gitInit.Dir = dir
	runCommand(t, gitInit)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("Hello, world!\n"), 0666))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("banana.txt\n"), 0666))

	gitAdd := exec.Command("git", "add", "README.md", ".gitignore")
	gitAdd.Dir = dir
	runCommand(t, gitAdd)

	gitCommit := exec.Command("git", "commit", "-m", "initial commit")
	gitCommit.Dir = dir
	runCommand(t, gitCommit)

	bakeConfiguration := new(tile.BakeConfiguration)
	require.NoError(t, bakeConfiguration.CalculateSourceChecksum(dir))

	firstChecksum, err := bakeConfiguration.Variable(tile.MetadataGitSHAVariable)
	require.NoError(t, err)
	assert.NotZero(t, firstChecksum)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "banana.txt"), []byte("🍌"), 0666))

	require.NoError(t, bakeConfiguration.CalculateSourceChecksum(dir))
	secondChecksum, err := bakeConfiguration.Variable(tile.MetadataGitSHAVariable)
	require.NoError(t, err)

	assert.Equal(t, firstChecksum, secondChecksum)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "apple.txt"), []byte("🍎"), 0666))

	require.Error(t, bakeConfiguration.CalculateSourceChecksum(dir))
}

func testdataTile(t *testing.T) string {
	return filepath.Join("testdata", "bake", path.Base(t.Name()))
}

func bakeTestTile(t *testing.T, tilePath string) fs.FS {
	t.Helper()
	_, err := os.Stat(tilePath)
	if err != nil {
		t.Fatal(err)
	}
	return os.DirFS(tilePath)
}

func compareLegacyMetadata(t *testing.T, tileDirPath string, bakeConfiguration *tile.BakeConfiguration) bool {
	t.Helper()

	legacyMetadataYAML := legacyBake(t, tileDirPath)
	var legacyMetadata proofing.ProductTemplate
	require.NoError(t, yaml.Unmarshal(legacyMetadataYAML, &legacyMetadata), string(legacyMetadataYAML))

	var metadataYAMLBuf bytes.Buffer
	err := tile.Metadata(&metadataYAMLBuf, bakeTestTile(t, tileDirPath), bakeConfiguration, nil)
	require.NoError(t, err)
	var metadata proofing.ProductTemplate
	require.NoError(t, yaml.Unmarshal(metadataYAMLBuf.Bytes(), &metadata), metadataYAMLBuf.String())

	return assert.Equal(t, legacyMetadata, metadata)
}

func legacyBake(t *testing.T, tilePath string, args ...string) []byte {
	t.Helper()

	var stdout, out bytes.Buffer
	kilnTest := exec.Command("kiln", append([]string{"bake"}, append(args, "--metadata-only", "--stub-releases")...)...)
	kilnTest.Dir = tilePath
	kilnTest.Stdout = io.MultiWriter(&stdout, &out)
	kilnTest.Stderr = &out
	if testing.Verbose() {
		logCommand(t, kilnTest)
	}
	if err := kilnTest.Run(); err != nil {
		if testing.Verbose() {
			t.Log(out.String())
		}
		t.Fatal(err)
	}
	return stdout.Bytes()
}

func runKilnTest(t *testing.T) {
	t.Helper()
	testdataTileName := path.Base(t.Name())

	t.Run("kiln test", func(t *testing.T) {
		t.SkipNow()
		if testing.Short() {
			t.Skip("skipping long running tests")
		}

		wd, err := os.Getwd()
		require.NoError(t, err)

		tileDir := filepath.Join(wd, "testdata", "bake", testdataTileName)

		require.NoError(t, os.RemoveAll(filepath.Join(tileDir, "vendor")))

		goModuleEditReplace := exec.Command("go", "mod", "edit", "-replace", "github.com/pivotal-cf/kiln="+filepath.Dir(filepath.Dir(wd)))
		goModuleEditReplace.Dir = tileDir
		runCommand(t, goModuleEditReplace)

		goModuleVendor := exec.Command("go", "mod", "vendor")
		goModuleVendor.Dir = tileDir
		runCommand(t, goModuleVendor)
		t.Cleanup(func() {
			_ = os.RemoveAll(filepath.Join(tileDir, "vendor"))
		})

		buildDir := t.TempDir()
		kilnBuild := filepath.Join(buildDir, "kiln")

		goBuild := exec.Command("go", "build", "-o", kilnBuild, "github.com/pivotal-cf/kiln")
		runCommand(t, goBuild)

		kilnTest := exec.Command(kilnBuild, "test")
		kilnTest.Dir = tileDir
		runCommand(t, kilnTest)
	})
}

func runCommand(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if testing.Verbose() {
		logCommand(t, cmd)
		cmd.Stderr = os.Stdout
		cmd.Stdout = os.Stdout
		err := cmd.Run()
		if err != nil {
			t.Error(err)
		}
		return
	}
	out := bytes.NewBuffer(nil)
	cmd.Stderr = out
	cmd.Stdout = out
	err := cmd.Run()
	if err != nil {
		t.Log(out.String())
		t.Error(err)
	}
}

func logCommand(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	t.Log("$ " + strings.Join(cmd.Args, " "))
}
