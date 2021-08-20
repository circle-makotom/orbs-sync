package depresolver

import (
	"log"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"

	"github.com/circle-makotom/orbs-sync/types"
)

var (
	orbRefMap       map[string]*types.VersionedOrb
	resolvedOrder   []*types.VersionedOrb
	dependenciesMap map[string]map[string]string
	dependentsMap   map[string]map[string]string

	logger = log.New(os.Stderr, "dependency-resolver: ", 7)
)

type orbImportingOrb struct {
	Orbs map[string]interface{}
}

func getDependents(orbRef string) map[string]string {
	dependents, ok := dependentsMap[orbRef]

	if !ok {
		dependents = make(map[string]string)
		dependentsMap[orbRef] = dependents
	}

	return dependents
}

func initMaps(orbs []*types.VersionedOrb) []string {
	illegible := []string{}

	for _, orb := range orbs {
		logger.Printf("initializing %q", orb.Ref)

		orbRefMap[orb.Ref] = orb

		dependencies := make(map[string]string)

		importingOrbs := &orbImportingOrb{}

		if err := yaml.Unmarshal([]byte(orb.Source), importingOrbs); err != nil {
			logger.Printf("ignoring orb %q because of YAML parser error: %v", orb.Ref, err.Error())
			illegible = append(illegible, orb.Ref)
		} else {
			if importingOrbs != nil {

				for procQueue := []map[string]interface{}{importingOrbs.Orbs}; len(procQueue) > 0; procQueue = procQueue[1:] {
					for _, prop := range procQueue[0] {
						switch value := prop.(type) {
						case string:
							siblingDependents := getDependents(value)
							siblingDependents[orb.Ref] = orb.Ref

							dependencies[value] = value
						case map[string]interface{}:
							if value["orbs"] != nil {
								if nestedOrbs, ok := value["orbs"].(map[string]interface{}); ok {
									procQueue = append(procQueue, nestedOrbs)
								}
							}
						}
					}
				}
			}

			dependenciesMap[orb.Ref] = dependencies
		}
	}

	return illegible
}

func listOrbsWithoutDependencies() []string {
	ret := []string{}

	for orbRef, dependencies := range dependenciesMap {
		if len(dependencies) == 0 {
			ret = append(ret, orbRef)
		}
	}

	return ret
}

func doDeleteReferencesForOrb(orbRef string) {
	delete(dependenciesMap, orbRef)

	if dependents, ok := dependentsMap[orbRef]; ok {
		for _, dependent := range dependents {
			delete(dependenciesMap[dependent], orbRef)
		}
	}
}

func trimMostInsignificantSemver(orbRef string) string {
	return regexp.MustCompile(`\.?\d+$`).ReplaceAllString(orbRef, "")
}

func deleteReferencesForOrb(orbRef string) {
	doDeleteReferencesForOrb(orbRef)

	// Gimmick: Trim insignificant semver portions to pick up dependents designating dependencies non-specifically
	// e.g., my-orb@x.y.z can be my-orb@x.y or my-orb@x
	for partialOrbRef := trimMostInsignificantSemver(orbRef); regexp.MustCompile(`\d+$`).MatchString(partialOrbRef); partialOrbRef = trimMostInsignificantSemver(partialOrbRef) {
		doDeleteReferencesForOrb(partialOrbRef)
	}

	// Gimmick: Deal with @volatile
	// e.g., my-orb@x.y.z can be my-orb@volatile
	doDeleteReferencesForOrb(regexp.MustCompile(`@[^@]*$`).ReplaceAllString(orbRef, "@volatile"))
}

func reduceDependenciesMap() map[string][]string {
	ret := make(map[string][]string)

	for orbRef, dependenciesMapEntry := range dependenciesMap {
		dependencies := []string{}

		for _, dependingOrb := range dependenciesMapEntry {
			dependencies = append(dependencies, dependingOrb)
		}

		ret[orbRef] = dependencies
	}

	return ret
}

func Resolve(orbs []*types.VersionedOrb) ([]*types.VersionedOrb, []string, map[string][]string, error) {
	orbRefMap = make(map[string]*types.VersionedOrb)
	resolvedOrder = []*types.VersionedOrb{}
	dependenciesMap = make(map[string]map[string]string)
	dependentsMap = make(map[string]map[string]string)

	illegible := initMaps(orbs)

	for {
		orbsWithoutDependencies := listOrbsWithoutDependencies()
		nProcessing := len(orbsWithoutDependencies)

		if nProcessing == 0 {
			break
		}

		for _, orbRef := range orbsWithoutDependencies {
			resolvedOrder = append(resolvedOrder, orbRefMap[orbRef])
			deleteReferencesForOrb(orbRef)
		}

		logger.Printf("resolver running; %d newly resolved, %d resolved in total, %d remaining\n", nProcessing, len(resolvedOrder), len(dependenciesMap))
	}

	logger.Printf("resolver done; %d resolved, %d unresolvable\n", len(resolvedOrder), len(dependenciesMap))

	return resolvedOrder, illegible, reduceDependenciesMap(), nil
}
