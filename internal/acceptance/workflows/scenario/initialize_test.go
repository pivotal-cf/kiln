package scenario

import (
	"context"
	"embed"
	"golang.org/x/exp/slices"
	"io/fs"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/cucumber/godog"
)

const testTilePath = "../hello-tile"

func TestInitialize(t *testing.T) {
	t.Run("shared", func(t *testing.T) {
		t.Run("Tile", func(t *testing.T) {
			sCtx := fakeScenarioContext{t}
			initializeTile(sCtx)
		})
		t.Run("TileSourceCode", func(t *testing.T) {
			sCtx := fakeScenarioContext{t}
			initializeTileSourceCode(sCtx)
		})
		t.Run("Exec", func(t *testing.T) {
			sCtx := fakeScenarioContext{t}
			initializeExec(sCtx)
		})
		t.Run("GitHub", func(t *testing.T) {
			sCtx := fakeScenarioContext{t}
			initializeGitHub(sCtx)
		})
	})

	t.Run("commands", func(t *testing.T) {
		t.Run("Bake", func(t *testing.T) {
			sCtx := fakeScenarioContext{t}
			initializeBake(sCtx)
		})
		t.Run("CacheCompiledReleases", func(t *testing.T) {
			sCtx := fakeScenarioContext{t}
			initializeCacheCompiledReleases(sCtx)
		})
		t.Run("Fetch", func(t *testing.T) {
			sCtx := fakeScenarioContext{t}
			initializeFetch(sCtx)
		})
		t.Run("FindRelease", func(t *testing.T) {
			sCtx := fakeScenarioContext{t}
			initializeFindReleaseVersion(sCtx)
		})
		t.Run("UpdateRelease", func(t *testing.T) {
			sCtx := fakeScenarioContext{t}
			initializeUpdateRelease(sCtx)
		})
		t.Run("UpdateStemcell", func(t *testing.T) {
			sCtx := fakeScenarioContext{t}
			initializeUpdatingStemcell(sCtx)
		})
		t.Run("ReleaseNotes", func(t *testing.T) {
			sCtx := fakeScenarioContext{t}
			initializeReleaseNotes(sCtx)
		})
		t.Run("Help", func(t *testing.T) {
			sCtx := fakeScenarioContext{t}
			initializeHelp(sCtx)
		})
	})

	t.Run("ensure all initialize functions are tested", func(t *testing.T) {
		goFiles, err := fs.Glob(thisPackage, "*.go")
		if err != nil {
			t.Errorf("failed to match initialize files: %s", err)
		}
		exportedDefs := regexp.MustCompile(`(?m)func (Initialize[^(\[]+).*godog\.ScenarioContext.*`)
		privateDefs := regexp.MustCompile(`(?m)func (initialize[^(\[]+).*scenarioContext.*`)

		calls := loadInvocationTestCalls(t)
		for _, goFile := range goFiles {
			goCode, err := thisPackage.ReadFile(goFile)
			if err != nil {
				t.Errorf("failed to match initialize files: %s", err)
			}
			exportedMatches := nthSubMatch(1, exportedDefs.FindAllStringSubmatch(string(goCode), -1))
			privateMatches := nthSubMatch(1, privateDefs.FindAllStringSubmatch(string(goCode), -1))
			if len(exportedMatches) == 0 {
				continue
			}

			for _, m := range exportedMatches {
				t.Run(m, func(t *testing.T) {
					privateName := strings.Replace(m, "I", "i", 1)
					if slices.Index(calls, privateName) == indexNotFound {
						t.Errorf("%s does not seem to be tested (please add a private function recieving scenarioContext and add a test to the initialize_test.go file)", m)
					}
					if slices.Index(privateMatches, privateName) == indexNotFound {
						t.Errorf("%s does not seem wrap a private testable function (please add it to initialize_test.go)", m)
					}
				})
			}

			testSortedAlphaNumerically(t, "the exported functions", exportedMatches)
			testSortedAlphaNumerically(t, "the private functions", privateMatches)
		}
	})
}

func testSortedAlphaNumerically(t *testing.T, name string, list []string) {
	t.Helper()
	if !sort.StringsAreSorted(list) {
		t.Errorf("%s are not sorted alphabetically", name)
		for _, el := range list {
			t.Logf("\t%s", el)
		}
	}
}

//go:embed *.go
var thisPackage embed.FS

func loadInvocationTestCalls(t *testing.T) []string {
	t.Helper()
	initializeTest, err := thisPackage.ReadFile("initialize_test.go")
	if err != nil {
		t.Errorf("failed to find the initialize test file: %s", err)
	}
	testCallExp := regexp.MustCompile(`(?m)^\s*(initialize\S+)\(sCtx\)$`)
	testCallMatches := testCallExp.FindAllStringSubmatch(string(initializeTest), -1)
	return nthSubMatch(1, testCallMatches)
}

func nthSubMatch(n int, matches [][]string) []string {
	result := make([]string, 0, len(matches))
	for _, call := range matches {
		result = append(result, call[n])
	}
	return result
}

// fakeScenarioContext constrains our use of Step on *godog.ScenarioContext.
// it does not fully check the expression arguments match the types for the func
// this is done by godog during execution this is just a quick check
type fakeScenarioContext struct {
	t *testing.T
}

var (
	contextType = reflect.TypeOf((*context.Context)(nil)).Elem()
	tableType   = reflect.TypeOf((*godog.Table)(nil))
)

// countRelevantParams counts params that are neither contextType as a first parameter
// nor tableType as an nth parameter
func countRelevantParams(ft reflect.Type) int {
	hasCTXParam := ft.NumIn() > 0 && ft.In(0) == contextType
	paramCount := ft.NumIn()
	if hasCTXParam {
		paramCount--
	}
	for i := 0; i < ft.NumIn(); i++ {
		if ft.In(i) == tableType {
			paramCount--
		}
	}
	return paramCount
}

func (fake fakeScenarioContext) Step(expr, stepFunc interface{}) {
	fn := reflect.ValueOf(stepFunc)
	if fn.Kind() != reflect.Func {
		fake.t.Errorf("expected stepFunc to be %s got %s", reflect.Func, fn.Kind())
	}
	ex, ok := expr.(*regexp.Regexp)
	if !ok {
		fake.t.Errorf("expected %#[1]v type %[1]T to be a %[2]T", expr, new(regexp.Regexp))
		return
	}
	ft := fn.Type()
	paramCount := countRelevantParams(ft)
	if ex.NumSubexp() != paramCount {
		fake.t.Errorf("expression %q has %d matches but function has %d params", ex, ex.NumSubexp(), paramCount)
	}
}

func (fake fakeScenarioContext) Before(godog.BeforeScenarioHook) {}

func (fake fakeScenarioContext) After(godog.AfterScenarioHook) {}

func isRunningInCI() bool {
	return os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTION") != ""
}
