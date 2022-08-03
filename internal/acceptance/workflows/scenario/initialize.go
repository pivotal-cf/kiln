package scenario

import (
	"github.com/cucumber/godog"
)

// InitializeContext is based on *godog.ScenarioContext
type InitializeContext interface {
	Step(expr, stepFunc interface{})
	Before(h godog.BeforeScenarioHook)
	After(h godog.AfterScenarioHook)
}

var _ InitializeContext = (*godog.ScenarioContext)(nil)
