package scenario

import (
	"context"
	"reflect"
	"regexp"
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
		t.Run("Help", func(t *testing.T) {
			sCtx := fakeScenarioContext{t}
			initializeHelp(sCtx)
		})
	})
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
