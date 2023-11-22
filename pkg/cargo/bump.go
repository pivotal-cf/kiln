package cargo

import (
	"context"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-github/v40/github"

	"github.com/pivotal-cf/kiln/internal/gh"
)

type Bump struct {
	Name, FromVersion, ToVersion string

	Releases []*github.RepositoryRelease
}

func (bump Bump) ReleaseNotes() string {
	var s strings.Builder

	for _, r := range bump.Releases {
		body := strings.TrimSpace(r.GetBody())
		if body == "" {
			continue
		}
		s.WriteString(body)
		s.WriteByte('\n')
	}

	return strings.TrimSpace(s.String())
}

func deduplicateReleasesWithTheSameTagName(bump Bump) Bump {
	updated := bump
	updated.Releases = slices.Clone(bump.Releases)
	for i, r := range updated.Releases {
		if i+1 >= len(updated.Releases) {
			break
		}
		for j := i + 1; j < len(updated.Releases); {
			after := updated.Releases[j]
			if r.GetTagName() != after.GetTagName() {
				j++
				continue
			}
			updated.Releases = append(updated.Releases[:j], updated.Releases[j+1:]...)
		}
	}
	return updated
}

func CalculateBumps(current, previous []BOSHReleaseTarballLock) []Bump {
	var (
		bumps         []Bump
		previousSpecs = make(map[string]BOSHReleaseTarballLock, len(previous))
	)
	for _, p := range previous {
		previousSpecs[p.Name] = p
	}
	for _, c := range current {
		p := previousSpecs[c.Name]
		if c.Version == p.Version {
			continue
		}
		bumps = append(bumps, Bump{
			Name:        c.Name,
			FromVersion: p.Version,
			ToVersion:   c.Version,
		})
	}
	return bumps
}

func (bump Bump) toFrom() (to, from *semver.Version, _ error) {
	var err error
	from, err = semver.NewVersion(bump.FromVersion)
	if err != nil {
		return nil, nil, err
	}
	to, err = semver.NewVersion(bump.ToVersion)
	if err != nil {
		return nil, nil, err
	}
	return to, from, err
}

type BumpList []Bump

func (list BumpList) ForLock(lock BOSHReleaseTarballLock) Bump {
	for _, b := range list {
		if b.Name == lock.Name {
			return b
		}
	}
	return Bump{
		Name:        lock.Name,
		FromVersion: lock.Version,
		ToVersion:   lock.Version,
	}
}

//counterfeiter:generate -o ./fakes_internal/repository_release_lister.go --fake-name RepositoryReleaseLister . repositoryReleaseLister

// repositoryReleaseLister is defined as not exported as a hack so counterfeiter does not add the
// type assignment at the end
type repositoryReleaseLister interface {
	ListReleases(ctx context.Context, owner, repo string, opts *github.ListOptions) ([]*github.RepositoryRelease, *github.Response, error)
}

type RepositoryReleaseLister = repositoryReleaseLister

func ReleaseNotes(ctx context.Context, repoService RepositoryReleaseLister, kf Kilnfile, list BumpList) (BumpList, error) {
	const workerCount = 10

	type fetchReleaseNotesForBump struct {
		bump  Bump
		index int
	}

	bumpFetcher := func(in <-chan fetchReleaseNotesForBump) <-chan fetchReleaseNotesForBump {
		results := make(chan fetchReleaseNotesForBump)

		go func() {
			defer close(results)
			wg := sync.WaitGroup{}
			defer wg.Wait()
			wg.Add(workerCount)
			for w := 0; w < workerCount; w++ {
				go func() {
					defer wg.Done()
					for j := range in {
						j.bump = fetchReleasesForBump(ctx, repoService, kf, j.bump)
						results <- j
					}
				}()
			}
		}()

		return results
	}

	c := make(chan fetchReleaseNotesForBump)

	results := bumpFetcher(c)

	go func() {
		for i, bump := range list {
			c <- fetchReleaseNotesForBump{
				index: i,
				bump:  bump,
			}
		}
		close(c)
	}()

	for r := range results {
		list[r.index].Releases = r.bump.Releases
	}

	return list, nil
}

func fetchReleasesFromRepo(ctx context.Context, repoService RepositoryReleaseLister, repository string, from, to *semver.Version) []*github.RepositoryRelease {
	owner, repo, err := gh.RepositoryOwnerAndNameFromPath(repository)
	if err != nil {
		return nil
	}

	var result []*github.RepositoryRelease

	ops := github.ListOptions{}
	releases, _, _ := repoService.ListReleases(ctx, owner, repo, &ops)

	for _, rel := range releases {
		rv, err := semver.NewVersion(strings.TrimPrefix(rel.GetTagName(), "v"))
		if err != nil || rv.LessThan(from) || rv.Equal(from) || rv.GreaterThan(to) {
			continue
		}
		result = append(result, rel)
	}

	return result
}

func fetchReleasesForBump(ctx context.Context, repoService RepositoryReleaseLister, kf Kilnfile, bump Bump) Bump {
	spec, err := kf.BOSHReleaseTarballSpecification(bump.Name)
	if err != nil {
		return bump
	}

	to, from, err := bump.toFrom()
	if err != nil {
		return bump
	}

	if spec.GitHubRepository != "" {
		releases := fetchReleasesFromRepo(ctx, repoService, spec.GitHubRepository, from, to)
		bump.Releases = append(bump.Releases, releases...)
	}

	sort.Sort(sort.Reverse(releasesByIncreasingSemanticVersion(bump.Releases)))
	bump = deduplicateReleasesWithTheSameTagName(bump)

	return bump
}

// releasesByIncreasingSemanticVersion sorts issues by increasing semantic version tags. If either release at
// i or j has a non semver tag, the existing ordering remains. So releases with improperly formatted semantic
// version tags continue to show up in a reasonable order.
type releasesByIncreasingSemanticVersion []*github.RepositoryRelease

func (list releasesByIncreasingSemanticVersion) Len() int { return len(list) }

func (list releasesByIncreasingSemanticVersion) Swap(i, j int) { list[i], list[j] = list[j], list[i] }

func (list releasesByIncreasingSemanticVersion) Less(i, j int) bool {
	it := list[i].GetTagName()
	iv, errIV := semver.NewVersion(strings.TrimPrefix(it, "v"))
	jt := list[j].GetTagName()
	jv, errJV := semver.NewVersion(strings.TrimPrefix(jt, "v"))
	if errIV != nil || errJV != nil {
		return i < j
	}
	return iv.LessThan(jv)
}
