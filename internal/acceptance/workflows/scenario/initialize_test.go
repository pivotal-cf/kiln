package scenario

import (
	"context"
	"embed"
	"io/fs"
	"os"
	"reflect"
	"regexp"
	"sort"
	"testing"

	"github.com/cucumber/godog"
	"golang.org/x/exp/slices"
)

const testTilePath = "../hello-tile"

func TestInitialize(t *testing.T) {
	t.Run("AWS", func(t *testing.T) {
		initializeAWS(newFakeScenarioContext(t))
	})
	t.Run("CacheCompiledReleases", func(t *testing.T) {
		initializeCacheCompiledReleases(newFakeScenarioContext(t))
	})
	t.Run("Env", func(t *testing.T) {
		initializeEnv(newFakeScenarioContext(t))
	})
	t.Run("Exec", func(t *testing.T) {
		initializeExec(newFakeScenarioContext(t))
	})
	t.Run("GitHub", func(t *testing.T) {
		initializeGitHub(newFakeScenarioContext(t))
	})
	t.Run("Kiln", func(t *testing.T) {
		initializeKiln(newFakeScenarioContext(t))
	})
	t.Run("RegEx", func(t *testing.T) {
		initializeRegex(newFakeScenarioContext(t))
	})
	t.Run("TanzuNetwork", func(t *testing.T) {
		initializeTanzuNetwork(newFakeScenarioContext(t))
	})
	t.Run("Tile", func(t *testing.T) {
		initializeTile(newFakeScenarioContext(t))
	})
	t.Run("TileSourceCode", func(t *testing.T) {
		initializeTileSourceCode(newFakeScenarioContext(t))
	})

	t.Run("ensure all initialize functions are tested", func(t *testing.T) {
		exportedInitializerFunctionPattern := regexp.MustCompile(`(?m)^func (Initialize[^(\[]+).*godog\.ScenarioContext.*`)
		privateInitializerFunctionPattern := regexp.MustCompile(`(?m)^func (initialize[^(\[]+).*scenarioContext.*`)

		testCalls := loadInvocationTestCalls(t)
		testSortedAlphaNumerically(t, "the test initialize function calls", testCalls)
		walkErr := fs.WalkDir(thisPackage, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			goCode, err := thisPackage.ReadFile(path)
			if err != nil {
				t.Errorf("failed to match initialize files: %s", err)
			}

			exportedInitializerDefinitionFunctionNames := nthSubMatch(1, exportedInitializerFunctionPattern.FindAllStringSubmatch(string(goCode), -1))
			privateInitializerDefinitionFunctionNames := nthSubMatch(1, privateInitializerFunctionPattern.FindAllStringSubmatch(string(goCode), -1))

			for _, exportedInitializerName := range exportedInitializerDefinitionFunctionNames {
				t.Run(exportedInitializerName, func(t *testing.T) {
					ensurePublicInitializerIsTested(t, exportedInitializerName, testCalls)
					ensurePublicInitializerHasPrivateImplementation(t, exportedInitializerName, privateInitializerDefinitionFunctionNames)
				})
			}

			testSortedAlphaNumerically(t, "the exported initialize functions", exportedInitializerDefinitionFunctionNames)
			testSortedAlphaNumerically(t, "the private initialize functions", privateInitializerDefinitionFunctionNames)

			return nil
		})
		if walkErr != nil {
			t.Errorf("error while walking go files: %s", walkErr)
		}
	})
}

func ensurePublicInitializerIsTested(t *testing.T, publicName string, calls []string) {
	t.Helper()
	privateName := privateInitializerName(publicName)
	if slices.Index(calls, privateName) == indexNotFound {
		t.Errorf("%s does not seem to be tested (please add a private function recieving scenarioContext and add a test to the initialize_test.go file)", publicName)
	}
}

func ensurePublicInitializerHasPrivateImplementation(t *testing.T, publicName string, privateMatches []string) {
	t.Helper()
	privateName := privateInitializerName(publicName)
	if slices.Index(privateMatches, privateName) == indexNotFound {
		t.Errorf("%s does not seem wrap a private testable function (please add it to initialize_test.go)", publicName)
	}
}

func privateInitializerName(public string) string {
	return "i" + public[1:]
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
	testCallExp := regexp.MustCompile(`(?m)^\s*(initialize\S+)\(newFakeScenarioContext\(t\)\)$`)
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

func isRunningInCI() bool {
	return os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTION") != ""
}

// fakeScenarioContext constrains our use of Step on *godog.ScenarioContext.
// it does not fully check the expression arguments match the types for the func
// this is done by godog during execution this is just a quick check
type fakeScenarioContext struct {
	t *testing.T
}

func newFakeScenarioContext(t *testing.T) fakeScenarioContext {
	t.Helper()
	return fakeScenarioContext{t: t}
}

var (
	contextType = reflect.TypeOf((*context.Context)(nil)).Elem()
	tableType   = reflect.TypeOf((*godog.Table)(nil))
)

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
