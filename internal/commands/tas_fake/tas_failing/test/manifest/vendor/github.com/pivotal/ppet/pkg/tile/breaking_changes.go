package tile

import (
	"fmt"

	"github.com/Masterminds/semver"
)

const (
	BreakingChangeAddedConfigurablePropertyWithoutDefault  = "added configurable property without default"
	BreakingChangedConfigurablePropertyToNotBeConfigurable = "changed configurable property to not be configurable"
	BreakingChangedConfigurablePropertyType                = "changed configurable property type"
	BreakingRemovedOrRenamedConfigurableProperty           = "removed or renamed configurable property"
	BreakingRemovedRemovedConfigurablePropertyDefault      = "removed configurable property default"
)

func ListBreakingChanges(stable, candidate Metadata) []error {
	var breakingChanges []error
	breakingChanges = append(breakingChanges, detectProductVersionErrors(stable, candidate)...)
	breakingChanges = append(breakingChanges, detectProductNameChange(stable, candidate)...)
	breakingChanges = append(breakingChanges, listPropertyBlueprintBreakingChanges(stable, candidate)...)
	breakingChanges = append(breakingChanges, detectRemovedErrand(stable, candidate)...)
	breakingChanges = append(breakingChanges, listJobDefinitionBreakingChanges(stable, candidate)...)

	return breakingChanges
}

func detectProductVersionErrors(stable, candidate Metadata) []error {
	var errList []error
	sv, err := semver.NewVersion(stable.ProductVersion)
	if err != nil {
		panic(fmt.Errorf("failed to parse stable product_version: %w", err))
	}

	cv, err := semver.NewVersion(candidate.ProductVersion)
	if err != nil {
		panic(fmt.Errorf("failed to parse stable product_version: %w", err))
	}
	if sv.Patch() != 0 {
		errList = append(errList, fmt.Errorf("stable metadata product_version patch number must be zero"))
	}
	if cv.LessThan(sv) {
		errList = append(errList, fmt.Errorf("stable metadata product_version must be less than candidate metadata product_version"))
	}

	return errList
}

func listPropertyBlueprintBreakingChanges(stable, candidate Metadata) []error {
	var breakingChanges []error
	for _, check := range []func(stable, candidate Metadata) []PropertyBlueprintBreakingChange{
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

func detectProductNameChange(stable, candidate Metadata) []error {
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

func detectNewPropertyWithoutDefault(stable, candidate Metadata) []PropertyBlueprintBreakingChange {
	var (
		newProperties   = listNewPropertyBlueprints(stable, candidate)
		breakingChanges []PropertyBlueprintBreakingChange
	)
	for _, pb := range newProperties {
		if !pb.IsConfigurable {
			continue
		}
		if pb.HasDefault() {
			continue
		}
		breakingChanges = append(breakingChanges, PropertyBlueprintBreakingChange{
			Type:         BreakingChangeAddedConfigurablePropertyWithoutDefault,
			PropertyName: pb.Name,
		})
	}
	return breakingChanges
}

func detectConfigurablePropertyChangedToNotBeConfigurable(stable, candidate Metadata) []PropertyBlueprintBreakingChange {
	var breakingChanges []PropertyBlueprintBreakingChange
	for _, candidateProperty := range candidate.PropertyBlueprints {
		stableProperty, _, found := stable.FindPropertyBlueprintWithName(candidateProperty.Name)
		if !found {
			continue
		}
		if stableProperty.IsConfigurable && !candidateProperty.IsConfigurable {
			breakingChanges = append(breakingChanges, PropertyBlueprintBreakingChange{
				Type:         BreakingChangedConfigurablePropertyToNotBeConfigurable,
				PropertyName: candidateProperty.Name,
			})
		}
	}
	return breakingChanges
}

func detectConfigurablePropertyTypeChanged(stable, candidate Metadata) []PropertyBlueprintBreakingChange {
	var breakingChanges []PropertyBlueprintBreakingChange
	for _, candidateProperty := range candidate.PropertyBlueprints {
		stableProperty, _, found := stable.FindPropertyBlueprintWithName(candidateProperty.Name)
		if !found ||
			!stableProperty.IsConfigurable || !candidateProperty.IsConfigurable {
			continue
		}
		if stableProperty.Type != candidateProperty.Type {
			breakingChanges = append(breakingChanges, PropertyBlueprintBreakingChange{
				Type:         BreakingChangedConfigurablePropertyType,
				PropertyName: candidateProperty.Name,
				Detail:       fmt.Sprintf("type changed from %q to %q", stableProperty.Type, candidateProperty.Type),
			})
		}
	}
	return breakingChanges
}

func detectRemovedConfigurableProperty(stable, candidate Metadata) []PropertyBlueprintBreakingChange {
	var breakingChanges []PropertyBlueprintBreakingChange
	for _, stableProperty := range stable.PropertyBlueprints {
		_, _, found := candidate.FindPropertyBlueprintWithName(stableProperty.Name)
		if !found {
			breakingChanges = append(breakingChanges, PropertyBlueprintBreakingChange{
				Type:         BreakingRemovedOrRenamedConfigurableProperty,
				PropertyName: stableProperty.Name,
			})
		}
	}
	return breakingChanges
}

func detectRemovedConfigurablePropertyDefault(stable, candidate Metadata) []PropertyBlueprintBreakingChange {
	var breakingChanges []PropertyBlueprintBreakingChange
	for _, candidateProperty := range candidate.PropertyBlueprints {
		stableProperty, _, found := stable.FindPropertyBlueprintWithName(candidateProperty.Name)
		if !found || !candidateProperty.IsConfigurable || !stableProperty.IsConfigurable {
			continue
		}
		if stableProperty.HasDefault() && !candidateProperty.HasDefault() {
			breakingChanges = append(breakingChanges, PropertyBlueprintBreakingChange{
				Type:         BreakingRemovedRemovedConfigurablePropertyDefault,
				PropertyName: stableProperty.Name,
			})
		}
	}
	return breakingChanges
}

func detectRemovedErrand(stable, candidate Metadata) []error {
	var breakingChanges []error
	for _, stableErrand := range stable.PostDeployErrands {
		found := candidate.HasPostDeployErrandWithName(stableErrand.Name)
		if !found {
			breakingChanges = append(breakingChanges, fmt.Errorf("breaking change for errand with name %q: removed", stableErrand.Name))
		}
	}
	return breakingChanges
}

func listJobDefinitionBreakingChanges(stable, candidate Metadata) []error {
	var breakingChanges []error
	for _, check := range []func(stable, candidate Metadata) []error{
		detectRemovedConfigurableInstanceGroup,
		detectConfigurabilityChangeToFalse,
		detectTightenedConstraints,
	} {
		breakingChanges = append(breakingChanges, check(stable, candidate)...)
	}
	return breakingChanges
}

func detectRemovedConfigurableInstanceGroup(stable, candidate Metadata) []error {
	var breakingChanges []error
	for _, stableIG := range stable.JobTypes {
		found := candidate.HasJobTypeWithName(stableIG.Name)
		if !found && stableIG.InstanceDefinition.Configurable {
			breakingChanges = append(breakingChanges, fmt.Errorf("breaking change for configurable instance group with name %q: removed", stableIG.Name))
		}
	}
	return breakingChanges
}

func detectConfigurabilityChangeToFalse(stable, candidate Metadata) []error {
	var breakingChanges []error
	for _, stableJob := range stable.JobTypes {
		candidateJob, found := candidate.FindJobTypeWithName(stableJob.Name)
		if found && stableJob.InstanceDefinition.Configurable && !candidateJob.InstanceDefinition.Configurable {
			breakingChanges = append(breakingChanges, fmt.Errorf("breaking change for configurable instance group with name %q: configurable changed to false", stableJob.Name))
		}
	}
	return breakingChanges
}

func detectTightenedConstraints(stable, candidate Metadata) []error {
	var breakingChanges []error
	for _, stableJob := range stable.JobTypes {
		candidateJob, found := candidate.FindJobTypeWithName(stableJob.Name)
		if !found {
			continue
		}

		oldMin := stableJob.InstanceDefinition.Constraints.Min
		newMin := candidateJob.InstanceDefinition.Constraints.Min
		oldMax := stableJob.InstanceDefinition.Constraints.Max
		newMax := candidateJob.InstanceDefinition.Constraints.Max
		if newMin > oldMin || newMax < oldMax {
			breakingChanges = append(breakingChanges, fmt.Errorf("breaking change for instance group with name %q: constraints tightened", stableJob.Name))
		}
	}
	return breakingChanges
}

func listNewPropertyBlueprints(stable, candidate Metadata) []PropertyBlueprint {
	var newProperties []PropertyBlueprint
	for _, candidatePropertyBlueprint := range candidate.PropertyBlueprints {
		_, _, found := stable.FindPropertyBlueprintWithName(candidatePropertyBlueprint.Name)
		if found {
			continue
		}
		newProperties = append(newProperties, candidatePropertyBlueprint)
	}
	return newProperties
}

func appendStaticErrorType[T error](errList []error, list ...T) []error {
	for _, c := range list {
		errList = append(errList, c)
	}
	return errList
}
