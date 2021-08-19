package bulkimporter

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"

	circleapi "github.com/CircleCI-Public/circleci-cli/api"
	circleql "github.com/CircleCI-Public/circleci-cli/api/graphql"

	"github.com/circle-makotom/orbs-sync/types"
)

var (
	maxImportRetries    = 3
	sleepBetweenRetries = 200 * time.Millisecond

	logger = log.New(os.Stderr, "bulk-importer: ", 7)
)

// Combination of OrbID and OrbExists
// Return a non-zero-length string, representing orb ID, if the orb exists, or a zero-length string if not
// Errors come from the underlying communication channel
//
// cf. https://github.com/CircleCI-Public/circleci-cli/blob/5297a1935de7cf25a0ee09b3a2baf5090ebc2020/api/api.go#L722-L758
// cf. https://github.com/CircleCI-Public/circleci-cli/blob/5297a1935de7cf25a0ee09b3a2baf5090ebc2020/api/api.go#L691-L719
func OrbIDUnsafe(cl *circleql.Client, orbName string) (string, error) {
	var response circleapi.OrbIDResponse

	query := `
		query ($name: String!) {
			orb(name: $name) {
				id
			}
		}
	`

	request := circleql.NewRequest(query)
	request.SetToken(cl.Token)
	request.Var("name", orbName)

	if err := cl.Run(request, &response); err != nil {
		return "", errors.Wrap(err, "GraphQL query failed")
	}

	return response.Orb.ID, nil
}

func ImportOrbsWithRetries(cl *circleql.Client, orbs []*types.VersionedOrb) ([]string, []string, error) {
	logger.Printf("importing listed orbs")

	availableOrbRefs := []string{}
	droppedOrbRefs := []string{}

	nsExists := make(map[string]bool)
	orbIDs := make(map[string]string)

	// cf. https://github.com/CircleCI-Public/circleci-cli/blob/5297a1935de7cf25a0ee09b3a2baf5090ebc2020/cmd/orb_import.go#L135-L167
	for _, orb := range orbs {
		var lastErr error

		logger.Printf("examining %q", orb.Ref)

		for iter := 0; iter < maxImportRetries; iter += 1 {
			logger.Printf("attempt %d of %d for %q", iter+1, maxImportRetries, orb.Ref)

			// cf. https://github.com/CircleCI-Public/circleci-cli/blob/5297a1935de7cf25a0ee09b3a2baf5090ebc2020/references/references.go#L10
			// cf. https://github.com/CircleCI-Public/circleci-cli/blob/5297a1935de7cf25a0ee09b3a2baf5090ebc2020/api/api.go#L454-L462
			orbNameParts := strings.Split(orb.Name, "/")
			ns := orbNameParts[0]
			shortname := strings.Join(orbNameParts[1:], "/")

			// Ensure that the namespace exists; create one if needed
			if _, nsVisited := nsExists[ns]; !nsVisited {
				// cf. https://github.com/CircleCI-Public/circleci-cli/blob/5297a1935de7cf25a0ee09b3a2baf5090ebc2020/cmd/orb_import.go#L98-L105
				doesExist, err := circleapi.NamespaceExists(cl, ns)
				if err != nil {
					lastErr = errors.Wrapf(err, "error while querying namespace %q", ns)
					time.Sleep(sleepBetweenRetries)
					continue
				}

				if !doesExist {
					// cf. https://github.com/CircleCI-Public/circleci-cli/blob/master/cmd/orb_import.go#L137-L140
					_, err := circleapi.CreateImportedNamespace(cl, ns)
					if err != nil {
						lastErr = errors.Wrapf(err, "error while creating namespace %q", ns)
						time.Sleep(sleepBetweenRetries)
						continue
					}

					logger.Printf("new namespace %q created", ns)
				}

				nsExists[ns] = true
				logger.Printf("cached namespace %q", ns)
			}

			// Ensure that the orb family is registered; register one if needed
			orbID, familyVisited := orbIDs[orb.Name]
			if !familyVisited {
				var err error

				// cf. https://github.com/CircleCI-Public/circleci-cli/blob/5297a1935de7cf25a0ee09b3a2baf5090ebc2020/cmd/orb_import.go#L109-L116
				orbID, err = OrbIDUnsafe(cl, orb.Name)
				if err != nil {
					lastErr = errors.Wrapf(err, "error while querying orb %q", ns)
					time.Sleep(sleepBetweenRetries)
					continue
				}

				if orbID == "" {
					// https://github.com/CircleCI-Public/circleci-cli/blob/5297a1935de7cf25a0ee09b3a2baf5090ebc2020/cmd/orb_import.go#L144-L147
					resp, err := circleapi.CreateImportedOrb(cl, ns, shortname)
					if err != nil {
						lastErr = errors.Wrapf(err, "error while registering orb %q", orb.Name)
						time.Sleep(sleepBetweenRetries)
						continue
					}
					orbID = resp.ImportOrb.Orb.ID

					logger.Printf("new orb %q registered with ID %q", orb.Name, orbID)
				}

				orbIDs[orb.Name] = orbID
				logger.Printf("cached orb %q with ID %q", orb.Name, orbID)
			}

			// Import the versioned orb if/only-if it is not imported yet
			// cf. https://github.com/CircleCI-Public/circleci-cli/blob/5297a1935de7cf25a0ee09b3a2baf5090ebc2020/cmd/orb_import.go#L120-L127
			_, err := circleapi.OrbInfo(cl, orb.Ref)
			if _, ok := err.(*circleapi.ErrOrbVersionNotExists); ok {
				logger.Printf("importing version %q of orb %q having ID %q", orb.Version, orb.Name, orbID)

				_, err = circleapi.OrbImportVersion(cl, orb.Source, orbID, orb.Version)
				if err != nil {
					msg := fmt.Sprintf("unable to publish versioned orb %q", orb.Ref)
					if strings.HasPrefix(err.Error(), "ERROR IN CONFIG FILE") {
						msg += "; possibly because the orb is using unsupported syntax for your server instance"
					}
					lastErr = errors.Wrap(err, msg)

					logger.Printf("error happend while importing %q", orb.Ref)
					logger.Println(lastErr)

					if iter+1 == maxImportRetries {
						logger.Printf("giving up to import %q; dropping it to continue", orb.Ref)
						droppedOrbRefs = append(droppedOrbRefs, orb.Ref)
						lastErr = nil

						break
					} else {
						time.Sleep(sleepBetweenRetries)
						continue
					}
				} else {
					logger.Printf("imported %q without errors", orb.Ref)
					availableOrbRefs = append(availableOrbRefs, orb.Ref)
					lastErr = nil

					break
				}
			} else if err != nil {
				lastErr = errors.Wrapf(err, "error while querying orb info %q", orb.Ref)
				time.Sleep(sleepBetweenRetries)
				continue
			} else {
				availableOrbRefs = append(availableOrbRefs, orb.Ref)
				lastErr = nil
				break
			}
		}

		if lastErr != nil {
			return nil, nil, errors.Wrapf(lastErr, "attempted import of %q %d time(s), but couldn't complete", orb.Ref, maxImportRetries)
		}
	}

	logger.Printf("import completed!")

	return availableOrbRefs, droppedOrbRefs, nil
}

func ImportOrbsWithNewClient(orbs []*types.VersionedOrb, hostname, apiEndpoint, token string, debug bool) ([]string, []string, error) {
	return ImportOrbsWithRetries(circleql.NewClient(&http.Client{}, hostname, apiEndpoint, token, debug), orbs)
}
