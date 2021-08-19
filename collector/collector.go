package collector

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	circleapi "github.com/CircleCI-Public/circleci-cli/api"
	circleql "github.com/CircleCI-Public/circleci-cli/api/graphql"

	"github.com/circle-makotom/orbs-sync/types"
)

const (
	fastStrategyBulkiness = 4

	// cf. https://github.com/CircleCI-Public/circleci-cli/blob/6ec121d68a6b12f46c604cc0f44d1e18d8bb2b52/api/api.go#L1424-L1448
	// cf. https://github.com/CircleCI-Public/circleci-cli/blob/6ec121d68a6b12f46c604cc0f44d1e18d8bb2b52/api/api.go#L1358-L1386
	listAllVersionedOrbsWithSrcQuery = `
	query ListOrbsWithAllVersions($first: Int!, $after: String!, $certifiedOnly: Boolean!) {
		orbs(first: $first, after: $after, certifiedOnly: $certifiedOnly) {
			totalCount
			edges {
				cursor
				node {
					name
					versions(count: 200) {
						version
						source
					}
				}
			}
			pageInfo {
				hasNextPage
			}
		}
	}
	`

	// cf. https://github.com/CircleCI-Public/circleci-cli/blob/6ec121d68a6b12f46c604cc0f44d1e18d8bb2b52/api/api.go#L1424-L1448
	// cf. https://github.com/CircleCI-Public/circleci-cli/blob/6ec121d68a6b12f46c604cc0f44d1e18d8bb2b52/api/api.go#L1358-L1386
	listAllVersionedOrbsWithoutSrcQuery = `
		query ListOrbsWithAllVersions($first: Int!, $after: String!, $certifiedOnly: Boolean!) {
			orbs(first: $first, after: $after, certifiedOnly: $certifiedOnly) {
				totalCount
				edges {
					cursor
					node {
						name
						versions(count: 200) {
							version
						}
					}
				}
				pageInfo {
					hasNextPage
				}
			}
		}
	`

	// cf. https://github.com/CircleCI-Public/circleci-cli/blob/5297a1935de7cf25a0ee09b3a2baf5090ebc2020/api/api.go#L722-L758
	// cf. https://github.com/CircleCI-Public/circleci-cli/blob/6ec121d68a6b12f46c604cc0f44d1e18d8bb2b52/api/api.go#L1358-L1386
	listVersionsWithSourceQuery = `
		query ($name: String!) {
			orb(name: $name) {
				name
				versions(count: 200) {
					version
					source
				}
			}
		}
	`

	// cf. https://github.com/CircleCI-Public/circleci-cli/blob/5297a1935de7cf25a0ee09b3a2baf5090ebc2020/api/api.go#L722-L758
	// cf. https://github.com/CircleCI-Public/circleci-cli/blob/6ec121d68a6b12f46c604cc0f44d1e18d8bb2b52/api/api.go#L1358-L1386
	//
	// This can be a duplicate of circleapi.OrbInfo, but we don't use that herein because that can raise untyped errors
	// cf. https://github.com/CircleCI-Public/circleci-cli/blob/6ec121d68a6b12f46c604cc0f44d1e18d8bb2b52/api/api.go#L1393
	listVersionsWithoutSourceQuery = `
		query ($name: String!) {
			orb(name: $name) {
				name
				versions(count: 200) {
					version
				}
			}
		}
	`
)

var logger = log.New(os.Stderr, "collector: ", 7)

type versionAPIResponse struct {
	Version string "json:\"version\""
	Source  string "json:\"source\""
}

type listVersionsForOneResponse struct {
	Orb struct {
		Name     string
		Versions []versionAPIResponse
	}
}

func processVersionedOrb(name string, version versionAPIResponse) *types.VersionedOrb {
	orbRef := fmt.Sprintf("%s@%s", name, version.Version)

	logger.Printf("discovered %q\n", orbRef)

	if err := yaml.Unmarshal([]byte(version.Source), &circleapi.OrbWithData{}); err != nil {
		logger.Printf(errors.Wrapf(err, "corrupt orb %q detected; skipping", orbRef).Error())
		return nil
	} else {
		return &types.VersionedOrb{
			Ref:     orbRef,
			Name:    name,
			Version: version.Version,
			Source:  version.Source,
		}
	}
}

func listKnownHiddenOrbs(cl *circleql.Client, targetOrbNames []string, includeSource bool) ([]*types.VersionedOrb, error) {
	ret := []*types.VersionedOrb{}

	logger.Printf("injecting known hidden orbs")

	for _, orbName := range targetOrbNames {
		var (
			versionedOrbs []*types.VersionedOrb
			err           error
		)

		logger.Printf("revealing %q", orbName)

		versionedOrbs, err = FetchVersionsForOne(cl, orbName, includeSource)

		if err != nil && errors.As(err, &circleapi.ErrOrbVersionNotExists{}) {
			return nil, errors.Wrapf(err, "could not list orbs %q", orbName)
		}

		ret = append(ret, versionedOrbs...)
	}

	return ret, nil
}

// cf. https://github.com/CircleCI-Public/circleci-cli/blob/6ec121d68a6b12f46c604cc0f44d1e18d8bb2b52/api/api.go#L1346-L1417
func FetchVersionsForOne(cl *circleql.Client, orbName string, includeSource bool) ([]*types.VersionedOrb, error) {
	ret := []*types.VersionedOrb{}

	var query string
	var response listVersionsForOneResponse

	if includeSource {
		logger.Printf("listing versions of orb %q with source\n", orbName)
		query = listVersionsWithSourceQuery
	} else {
		logger.Printf("listing versions of orb %q without source\n", orbName)
		query = listVersionsWithoutSourceQuery
	}

	request := circleql.NewRequest(query)
	request.SetToken(cl.Token)
	request.Var("name", orbName)

	if err := cl.Run(request, &response); err != nil {
		return nil, errors.Wrap(err, "GraphQL query failed")
	}

	for _, version := range response.Orb.Versions {
		if versionedOrb := processVersionedOrb(response.Orb.Name, version); versionedOrb != nil {
			ret = append(ret, versionedOrb)
		}
	}

	return ret, nil
}

// cf. https://github.com/CircleCI-Public/circleci-cli/blob/6ec121d68a6b12f46c604cc0f44d1e18d8bb2b52/api/api.go#L1419-L1492
func ListAllVersionedOrbsFast(cl *circleql.Client, knownHiddenOrbs []string, includeSource, includeUncertified bool) ([]*types.VersionedOrb, error) {
	var query string

	// Gimmick: Manually list known hidden orbs, including welcome orbs; these are hidden orbs, although referenced often
	ret, err := listKnownHiddenOrbs(cl, knownHiddenOrbs, includeSource)
	if err != nil {
		return nil, err
	}

	if includeSource {
		query = listAllVersionedOrbsWithSrcQuery
	} else {
		query = listAllVersionedOrbsWithoutSrcQuery
	}

	logger.Printf("fetching all versioned orbs at once")
	currentCursor := ""
	for {
		var result circleapi.OrbListResponse

		request := circleql.NewRequest(query)
		request.Var("first", fastStrategyBulkiness)
		request.Var("after", currentCursor)
		request.Var("certifiedOnly", !includeUncertified)

		if err := cl.Run(request, &result); err != nil {
			return nil, errors.Wrap(err, "GraphQL query failed")
		}

		for _, edge := range result.Orbs.Edges {
			currentCursor = edge.Cursor

			for _, version := range edge.Node.Versions {
				if versionedOrb := processVersionedOrb(edge.Node.Name, version); versionedOrb != nil {
					ret = append(ret, versionedOrb)
				}
			}
		}

		if !result.Orbs.PageInfo.HasNextPage {
			break
		}
	}

	return ret, nil
}

// cf. https://github.com/CircleCI-Public/circleci-cli/blob/6ec121d68a6b12f46c604cc0f44d1e18d8bb2b52/api/api.go#L1419-L1492
func ListAllVersionedOrbsSlow(cl *circleql.Client, knownHiddenOrbs []string, includeSource, includeUncertified bool) ([]*types.VersionedOrb, error) {
	var ret []*types.VersionedOrb

	// Gimmick: Manually list known hidden orbs, including welcome orbs; these are hidden orbs, although referenced often
	ret, err := listKnownHiddenOrbs(cl, knownHiddenOrbs, includeSource)
	if err != nil {
		return nil, err
	}

	logger.Printf("listing all orb names")
	orbList, err := circleapi.ListOrbs(cl, includeUncertified)
	if err != nil {
		return nil, errors.Wrap(err, "error while listing orbs")
	}

	logger.Printf("fetching all versions of each orb")
	for _, orb := range orbList.Orbs {
		logger.Printf("working on %q", orb.Name)

		versionedOrbs, err := FetchVersionsForOne(cl, orb.Name, includeSource)

		if err != nil {
			// FetchVersionedOrbs can fail if the source of orb is astonishingly big
			// As a fallback fetch each version one-by-one herein
			// This operation can be astronomically slow however
			logger.Printf("oof, could not fetch versions of orb %q at once; trying to fetch each version one-by-one", orb.Name)

			logger.Printf("listing all versions of orb %q without source", orb.Name)
			orbVersions, err := FetchVersionsForOne(cl, orb.Name, false)
			if err != nil {
				return nil, err
			}

			for _, orbVersion := range orbVersions {
				logger.Printf("fetching source of orb %s", orbVersion.Ref)
				orbSrc, err := circleapi.OrbSource(cl, orbVersion.Ref)
				if err != nil {
					return nil, errors.Wrapf(err, "could not fetch source of orb %q", orbVersion.Ref)
				}

				ret = append(ret, &types.VersionedOrb{
					Ref:     orbVersion.Ref,
					Name:    orbVersion.Name,
					Version: orbVersion.Version,
					Source:  orbSrc,
				})
			}
		} else {
			ret = append(ret, versionedOrbs...)
		}
	}

	return ret, nil
}

func ListAllVersionedOrbsWithNewClient(hostname, apiEndpoint, token string, knownHiddenOrbs []string, includeSource, includeUncertified, beSlow, debug bool) ([]*types.VersionedOrb, error) {
	cl := circleql.NewClient(&http.Client{}, hostname, apiEndpoint, token, debug)

	if beSlow {
		return ListAllVersionedOrbsSlow(cl, knownHiddenOrbs, includeSource, includeUncertified)
	} else {
		return ListAllVersionedOrbsFast(cl, knownHiddenOrbs, includeSource, includeUncertified)
	}
}
