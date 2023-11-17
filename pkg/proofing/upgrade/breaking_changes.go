package upgrade

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/pivotal-cf/kiln/pkg/proofing"
)

const (
	BreakingChangeAddedConfigurablePropertyWithoutDefault  = "added configurable property without default"
	BreakingChangedConfigurablePropertyToNotBeConfigurable = "changed configurable property to not be configurable"
	BreakingChangedConfigurablePropertyType                = "changed configurable property type"
	BreakingRemovedOrRenamedConfigurableProperty           = "removed or renamed configurable property"
	BreakingRemovedRemovedConfigurablePropertyDefault      = "removed configurable property default"
)

func ListBreakingChanges(stable, candidate proofing.ProductTemplate) []error {
	var breakingChanges []error
	breakingChanges = append(breakingChanges, detectProductVersionErrors(stable, candidate)...)
	breakingChanges = append(breakingChanges, detectProductNameChange(stable, candidate)...)
	breakingChanges = append(breakingChanges, listPropertyBlueprintBreakingChanges(stable, candidate)...)
	breakingChanges = append(breakingChanges, detectRemovedErrand(stable, candidate)...)
	breakingChanges = append(breakingChanges, listJobDefinitionBreakingChanges(stable, candidate)...)

	return breakingChanges
}

func detectProductVersionErrors(stable, candidate proofing.ProductTemplate) []error {
	var errList []error
	sv, err := semver.NewVersion(stable.ProductVersion)
	if err != nil {
		errList = append(errList, fmt.Errorf("failed to parse stable product_version: %w", err))
		return errList
	}

	cv, err := semver.NewVersion(candidate.ProductVersion)
	if err != nil {
		errList = append(errList, fmt.Errorf("failed to parse candidate product_version: %w", err))
		return errList
	}
	if sv.Patch() != 0 {
		errList = append(errList, fmt.Errorf("stable metadata product_version patch number must be zero"))
	}
	if cv.LessThan(sv) {
		errList = append(errList, fmt.Errorf("stable metadata product_version must be less than candidate metadata product_version"))
	}

	return errList
}

func listPropertyBlueprintBreakingChanges(stable, candidate proofing.ProductTemplate) []error {
	var breakingChanges []error
	for _, check := range []func(stable, candidate proofing.ProductTemplate) []PropertyBlueprintBreakingChange{
		detectNewPropertyWithoutDefault,
		detectConfigurablePropertyChangedToNotBeConfigurable,
		detectConfigurablePropertyTypeChanged,
		detectRemovedConfigurableProperty,
		detectRemovedConfigurablePropertyDefault,
	} {
		breakingChanges = appendStaticErrorType(breakingChanges, check(stable, candidate)...)
	}
	return breakingChanges
}

func detectProductNameChange(stable, candidate proofing.ProductTemplate) []error {
	if stable.Name != candidate.Name {
		return []error{
			fmt.Errorf("breaking change tile names are not the same (%q != %q)", stable.Name, candidate.Name),
		}
	}
	return nil
}

type PropertyBlueprintBreakingChange struct {
	Type, Detail,
	PropertyName string
}

func (bc PropertyBlueprintBreakingChange) Error() string {
	var detailString string
	if bc.Detail != "" {
		detailString += ": " + bc.Detail
	}
	return fmt.Sprintf("breaking change for property with name %q: %s%s", bc.PropertyName, bc.Type, detailString)
}

func detectNewPropertyWithoutDefault(stable, candidate proofing.ProductTemplate) []PropertyBlueprintBreakingChange {
	var (
		newProperties   = listNewPropertyBlueprints(stable, candidate)
		breakingChanges []PropertyBlueprintBreakingChange
	)
	for _, pb := range newProperties {
		if !pb.IsConfigurable() {
			continue
		}
		if pb.HasDefault() {
			continue
		}
		if pb.IsOptional() {
			continue
		}
		breakingChanges = append(breakingChanges, PropertyBlueprintBreakingChange{
			Type:         BreakingChangeAddedConfigurablePropertyWithoutDefault,
			PropertyName: pb.PropertyName(),
		})
	}
	return breakingChanges
}

func detectConfigurablePropertyChangedToNotBeConfigurable(stable, candidate proofing.ProductTemplate) []PropertyBlueprintBreakingChange {
	var breakingChanges []PropertyBlueprintBreakingChange
	for _, candidateProperty := range candidate.PropertyBlueprints {
		stableProperty, _, err := stable.FindPropertyBlueprintWithName(candidateProperty.PropertyName())
		if err != nil {
			continue
		}
		if stableProperty.IsConfigurable() && !candidateProperty.IsConfigurable() {
			breakingChanges = append(breakingChanges, PropertyBlueprintBreakingChange{
				Type:         BreakingChangedConfigurablePropertyToNotBeConfigurable,
				PropertyName: candidateProperty.PropertyName(),
			})
		}
	}
	return breakingChanges
}

func detectConfigurablePropertyTypeChanged(stable, candidate proofing.ProductTemplate) []PropertyBlueprintBreakingChange {
	var breakingChanges []PropertyBlueprintBreakingChange
	for _, candidateProperty := range candidate.PropertyBlueprints {
		stableProperty, _, err := stable.FindPropertyBlueprintWithName(candidateProperty.PropertyName())
		if err != nil ||
			!stableProperty.IsConfigurable() || !candidateProperty.IsConfigurable() {
			continue
		}
		if stableProperty.PropertyType() != candidateProperty.PropertyType() {
			breakingChanges = append(breakingChanges, PropertyBlueprintBreakingChange{
				Type:         BreakingChangedConfigurablePropertyType,
				PropertyName: candidateProperty.PropertyName(),
				Detail:       fmt.Sprintf("type changed from %q to %q", stableProperty.PropertyType(), candidateProperty.PropertyType()),
			})
		}
	}
	return breakingChanges
}

func detectRemovedConfigurableProperty(stable, candidate proofing.ProductTemplate) []PropertyBlueprintBreakingChange {
	var breakingChanges []PropertyBlueprintBreakingChange
	for _, stableProperty := range stable.PropertyBlueprints {
		_, _, err := candidate.FindPropertyBlueprintWithName(stableProperty.PropertyName())
		if err != nil {
			breakingChanges = append(breakingChanges, PropertyBlueprintBreakingChange{
				Type:         BreakingRemovedOrRenamedConfigurableProperty,
				PropertyName: stableProperty.PropertyName(),
			})
		}
	}
	return breakingChanges
}

func detectRemovedConfigurablePropertyDefault(stable, candidate proofing.ProductTemplate) []PropertyBlueprintBreakingChange {
	var breakingChanges []PropertyBlueprintBreakingChange
	for _, candidateProperty := range candidate.PropertyBlueprints {
		stableProperty, _, findErr := stable.FindPropertyBlueprintWithName(candidateProperty.PropertyName())
		if findErr != nil || !candidateProperty.IsConfigurable() || !stableProperty.IsConfigurable() {
			continue
		}
		if stableProperty.HasDefault() && !candidateProperty.HasDefault() {
			breakingChanges = append(breakingChanges, PropertyBlueprintBreakingChange{
				Type:         BreakingRemovedRemovedConfigurablePropertyDefault,
				PropertyName: stableProperty.PropertyName(),
			})
		}
	}
	return breakingChanges
}

func detectRemovedErrand(stable, candidate proofing.ProductTemplate) []error {
	var breakingChanges []error
	for _, stableErrand := range stable.PostDeployErrands {
		found := candidate.HasPostDeployErrandWithName(stableErrand.Name)
		if !found {
			breakingChanges = append(breakingChanges, fmt.Errorf("breaking change for errand with name %q: removed", stableErrand.Name))
		}
	}
	return breakingChanges
}

func listJobDefinitionBreakingChanges(stable, candidate proofing.ProductTemplate) []error {
	var breakingChanges []error
	for _, check := range []func(stable, candidate proofing.ProductTemplate) []error{
		detectRemovedConfigurableInstanceGroup,
		detectConfigurabilityChangeToFalse,
		detectTightenedInstanceGroupConstraints,
	} {
		breakingChanges = append(breakingChanges, check(stable, candidate)...)
	}
	return breakingChanges
}

func detectRemovedConfigurableInstanceGroup(stable, candidate proofing.ProductTemplate) []error {
	var breakingChanges []error
	for _, stableIG := range stable.JobTypes {
		found := candidate.HasJobTypeWithName(stableIG.Name)
		if !found && stableIG.InstanceDefinition.Configurable {
			breakingChanges = append(breakingChanges, fmt.Errorf("breaking change for configurable instance group with name %q: removed", stableIG.Name))
		}
	}
	return breakingChanges
}

func detectConfigurabilityChangeToFalse(stable, candidate proofing.ProductTemplate) []error {
	var breakingChanges []error
	for _, stableJob := range stable.JobTypes {
		candidateJob, _, findErr := candidate.FindJobTypeWithName(stableJob.Name)
		if findErr == nil && stableJob.InstanceDefinition.Configurable && !candidateJob.InstanceDefinition.Configurable {
			breakingChanges = append(breakingChanges, fmt.Errorf("breaking change for configurable instance group with name %q: configurable changed to false", stableJob.Name))
		}
	}
	return breakingChanges
}

func detectTightenedInstanceGroupConstraints(stable, candidate proofing.ProductTemplate) []error {
	var breakingChanges []error
	for _, stableJob := range stable.JobTypes {
		candidateJob, _, err := candidate.FindJobTypeWithName(stableJob.Name)
		if err != nil {
			continue
		}
		for _, constraintErr := range checkTighterConstraints(stableJob.InstanceDefinition, candidateJob.InstanceDefinition) {
			breakingChanges = append(breakingChanges, fmt.Errorf("breaking change for instance definition constraint with name %q: %w", stableJob.Name, constraintErr))
		}
	}
	return breakingChanges
}

// checkTighterConstraints only check proofing.IntegerConstraints.Min and proofing.IntegerConstraints.Max
// TODO: implement checks for other fields
func checkTighterConstraints(stableJob, candidateJob proofing.InstanceDefinition) []error {
	if candidateJob.Constraints == nil {
		return nil
	}
	var errList []error
	if stableJob.Constraints != nil && candidateJob.Constraints.Max != nil && ((stableJob.Constraints.Max == nil) || (*candidateJob.Constraints.Max < *stableJob.Constraints.Max)) {
		errList = append(errList, fmt.Errorf("reduced max constraint"))
	}
	if stableJob.Constraints != nil && candidateJob.Constraints.Min != nil && ((stableJob.Constraints.Min == nil) || (*candidateJob.Constraints.Min > *stableJob.Constraints.Min)) {
		errList = append(errList, fmt.Errorf("increased min constraint"))
	}
	return errList
}

func listNewPropertyBlueprints(stable, candidate proofing.ProductTemplate) []proofing.PropertyBlueprint {
	var newProperties []proofing.PropertyBlueprint
	for _, candidatePropertyBlueprint := range candidate.PropertyBlueprints {
		_, _, err := stable.FindPropertyBlueprintWithName(candidatePropertyBlueprint.PropertyName())
		if err != nil && err.Error() == "not found" {
			newProperties = append(newProperties, candidatePropertyBlueprint)
		}
	}
	return newProperties
}

func appendStaticErrorType[T error](errList []error, list ...T) []error {
	for _, c := range list {
		errList = append(errList, c)
	}
	return errList
}
