package scenario

import (
	"github.com/cucumber/godog"
)

// initializeContext is based on *godog.ScenarioContext
type initializeContext interface {
	Step(expr, stepFunc interface{})
	Before(h godog.BeforeScenarioHook)
	After(h godog.AfterScenarioHook)
}

// initializeContext exposes the subset of methods on *godog.ScenarioContext that we use.
// It is here because we want to have a bit of testing for the initialize functions.
var _ initializeContext = (*godog.ScenarioContext)(nil)
