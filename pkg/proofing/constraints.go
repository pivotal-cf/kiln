package proofing

import (
	"fmt"
	"math/bits"
)

type IntegerConstraints struct {
	Min       *int `yaml:"min"`
	Max       *int `yaml:"max"`
	ZeroOrMin *int `yaml:"zero_or_min"`
	Modulo    *int `yaml:"modulo"`

	PowerOfTwo         *bool `yaml:"power_of_two"`
	MayOnlyIncrease    *bool `yaml:"may_only_increase"`
	MayOnlyBeOddOrZero *bool `yaml:"may_only_be_odd_or_zero"`
}

func (constraints IntegerConstraints) CheckValue(value int) error {
	return ensureEach(constraints, value,
		noopIfNil(constraints.Min, IntegerConstraints.ensureValueIsBelowMin),
		noopIfNil(constraints.Max, IntegerConstraints.ensureValueIsAboveMax),
		noopIfNil(constraints.MayOnlyBeOddOrZero, IntegerConstraints.ensureValueMayOnlyBeOddOrZero),
		noopIfNil(constraints.ZeroOrMin, IntegerConstraints.ensureValueZeroOrGreaterThanMin),
		noopIfNil(constraints.Modulo, IntegerConstraints.ensureValueIsModulo),
		noopIfNil(constraints.PowerOfTwo, IntegerConstraints.ensureValueIsPowerOfTwo),
	)
}

func (IntegerConstraints) ensureValueIsBelowMin(min, value int) error {
	if value < min {
		return fmt.Errorf("value %d must be greater than or equal to %d", value, min)
	}
	return nil
}

func (IntegerConstraints) ensureValueIsAboveMax(max, value int) error {
	if value > max {
		return fmt.Errorf("value %d must be less than or equal to %d", value, max)
	}
	return nil
}

func (IntegerConstraints) ensureValueMayOnlyBeOddOrZero(mayOnlyBeOddOrZero bool, value int) error {
	if mayOnlyBeOddOrZero && value%2 == 0 && value != 0 {
		return fmt.Errorf("value %d must be odd or zero", value)
	}
	return nil
}

func (IntegerConstraints) ensureValueZeroOrGreaterThanMin(min int, value int) error {
	if value < min && value != 0 {
		return fmt.Errorf("value %d must zero or at least %d", value, min)
	}
	return nil
}

func (IntegerConstraints) ensureValueIsModulo(mod, value int) error {
	if value%mod != 0 {
		return fmt.Errorf("value %d must be modulo of %d", value, mod)
	}
	return nil
}

func (IntegerConstraints) ensureValueIsPowerOfTwo(mustBePowerOfTwo bool, value int) error {
	if mustBePowerOfTwo && bits.OnesCount(uint(value)) > 1 {
		return fmt.Errorf("value %d must be a power of two", value)
	}
	return nil
}

func ensureEach[Constraint any, Value any](c Constraint, value Value, ensureFunctions ...func(Constraint, Value) error) error {
	for _, fn := range ensureFunctions {
		if err := fn(c, value); err != nil {
			return err
		}
	}
	return nil
}

func noopValidateFunc[Constraint any, V any](Constraint, V) error { return nil }

func noopIfNil[Constraint any, Field any, V any](field *Field, fn func(c Constraint, field Field, value V) error) func(Constraint, V) error {
	if field == nil {
		return noopValidateFunc[Constraint, V]
	}
	return func(constraint Constraint, v V) error {
		return fn(constraint, *field, v)
	}
}
