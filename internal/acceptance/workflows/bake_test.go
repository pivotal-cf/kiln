package workflows

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/cucumber/godog"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

var success error = nil

func TestBake(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: initializeBakeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			TestingT: t, // Testing instance that will run subtests.
		},
	}

	if code := suite.Run(); code != 0 {
		t.Fatalf("status %d returned, failed to run feature tests", code)
	}
}

func initializeBakeScenario(ctx *godog.ScenarioContext) {
	var td kilnBakeScenario

	td.loadGithubToken()

	ctx.Step(regexp.MustCompile(`^a Tile is created$`), td.aTileIsCreated)
	ctx.Step(regexp.MustCompile(`^I have a "([^"]*)" repository checked out at (.*)$`), td.iHaveARepositoryCheckedOutAtRevision)
	ctx.Step(regexp.MustCompile(`^I invoke kiln bake$`), td.iInvokeKilnBake)
	ctx.Step(regexp.MustCompile(`^the Tile contains "([^"]*)"$`), td.theTileContains)
	ctx.Step(regexp.MustCompile(`^I fetch releases$`), td.iFetchReleases)
}

// kilnBakeScenario
type kilnBakeScenario struct {
	tilePath, tileVersion string
	githubToken           string
}

// aTileIsCreated asserts the output tile exists
func (scenario *kilnBakeScenario) aTileIsCreated() error {
	_, err := os.Stat(scenario.defaultFilePathForTile())
	return err
}

// iFetchReleases fetches releases. It provides the command with the GitHub token (used for hello-release).
func (scenario *kilnBakeScenario) iFetchReleases() error {
	cmd := exec.Command("go", "run", "github.com/pivotal-cf/kiln", "fetch", "--variable", "github_token="+scenario.githubToken)
	cmd.Dir = scenario.tilePath
	// TODO: only log output if run fails
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// iHaveARepositoryCheckedOutAtRevision checks out a repository at the filepath to a given revision
// Importantly, it also sets tilePath and tileVersion on kilnBakeScenario.
func (scenario *kilnBakeScenario) iHaveARepositoryCheckedOutAtRevision(filePath, revision string) error {
	repo, err := git.PlainOpen(filePath)
	if err != nil {
		return fmt.Errorf("opening the repository failed: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("loading the worktree failed: %w", err)
	}

	revisionHash, err := repo.ResolveRevision(plumbing.Revision(revision))
	if err != nil {
		return fmt.Errorf("resolving the given revision %q failed: %w", revision, err)
	}

	err = wt.Checkout(&git.CheckoutOptions{
		Hash:  *revisionHash,
		Force: true,
	})
	if err != nil {
		return fmt.Errorf("checking out the revision %q at %q failed: %w", revision, revisionHash, err)
	}

	scenario.tilePath = filePath
	scenario.tileVersion = strings.TrimPrefix(revision, "v")

	return success
}

// iInvokeKilnBake invokes kiln bake with tileVersion provided by iHaveARepositoryCheckedOutAtRevision
func (scenario *kilnBakeScenario) iInvokeKilnBake() error {
	cmd := exec.Command("go", "run", "github.com/pivotal-cf/kiln", "bake", "--version", scenario.tileVersion)
	cmd.Dir = scenario.tilePath
	// TODO: only log output if run fails
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// theTileContains checks that the filePaths exist in the tile
func (scenario *kilnBakeScenario) theTileContains(_ string, table *godog.Table) error {
	tile, err := zip.OpenReader(scenario.defaultFilePathForTile())
	if err != nil {
		return err
	}
	for _, row := range table.Rows {
		for _, cell := range row.Cells {
			_, err := tile.Open(cell.Value)
			if err != nil {
				return fmt.Errorf("tile did not contain file %s", cell.Value)
			}
		}
	}
	return success
}

// defaultFilePathForTile returns a path based on the default output tile value of bake
func (scenario *kilnBakeScenario) defaultFilePathForTile() string {
	return filepath.Join(scenario.tilePath, "tile-"+scenario.tileVersion+".pivotal")
}

func (scenario *kilnBakeScenario) loadGithubToken() {
	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		cmd := exec.Command("gh", "auth", "status", "--show-token")
		var out bytes.Buffer
		cmd.Stderr = &out
		err := cmd.Run()
		if err != nil {
			panic("login to github using the CLI or set GITHUB_TOKEN")
		}
		matches := regexp.MustCompile("(?m)^.*Token: (gho_.*)$").FindStringSubmatch(out.String())
		if len(matches) == 0 {
			panic("login to github using the CLI or set GITHUB_TOKEN")
		}
		githubToken = matches[1]
	}
	scenario.githubToken = githubToken
}
