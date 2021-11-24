package commands

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/go-git/go-git/v5"
	"github.com/google/go-github/v40/github"

	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

func getGithubRemoteRepoOwnerAndName(repo *git.Repository) (string, string, error) {
	var remoteURL string
	remote, err := repo.Remote("origin")
	if err != nil {
		return "", "", err
	}
	config := remote.Config()
	for _, u := range config.URLs {
		if !strings.Contains(u, "github.com") {
			continue
		}
		remoteURL = u
		break
	}
	if remoteURL == "" {
		return "", "", fmt.Errorf("remote github URL not found for repo")
	}

	repoOwner, repoName, err := component.OwnerAndRepoFromGitHubURI(remoteURL)
	if err != nil {
		return "", "", err
	}

	return repoOwner, repoName, nil
}

type BoshReleaseBump struct {
	Name                   string
	FromVersion, ToVersion string
	Releases               []*github.RepositoryRelease
}

func (bump BoshReleaseBump) ReleaseNotes() string {
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

func (bump *BoshReleaseBump) deduplicateReleasesWithTheSameTagName() {
	for i, r := range bump.Releases {
		if i+1 >= len(bump.Releases) {
			break
		}
		for j := i + 1; j < len(bump.Releases); {
			after := bump.Releases[j]
			if r.GetTagName() != after.GetTagName() {
				j++
				continue
			}
			bump.Releases = append(bump.Releases[:j], bump.Releases[j+1:]...)
		}
	}
}

func calculateComponentBumps(current, previous []component.Lock) []BoshReleaseBump {
	var (
		bumps         []BoshReleaseBump
		previousSpecs = make(map[string]component.Lock, len(previous))
	)
	for _, p := range previous {
		previousSpecs[p.Name] = p
	}
	for _, c := range current {
		p := previousSpecs[c.Name]
		if c.Version == p.Version {
			continue
		}
		bumps = append(bumps, BoshReleaseBump{
			Name:        c.Name,
			FromVersion: p.Version,
			ToVersion:   c.Version,
		})
	}
	return bumps
}

func (bump BoshReleaseBump) toFrom() (to, from *semver.Version, _ error) {
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

type BumpList []BoshReleaseBump

func (list BumpList) ForLock(lock component.Lock) BoshReleaseBump {
	for _, b := range list {
		if b.Name == lock.Name {
			return b
		}
	}
	return BoshReleaseBump{
		Name:        lock.Name,
		FromVersion: lock.Version,
		ToVersion:   lock.Version,
	}
}

//counterfeiter:generate -o ./fakes_internal/release_lister.go --fake-name ReleaseLister . releaseLister

type releaseLister interface {
	ListReleases(ctx context.Context, owner, repo string, opts *github.ListOptions) ([]*github.RepositoryRelease, *github.Response, error)
}

func fetchReleaseNotes(ctx context.Context, repoService releaseLister, kf cargo.Kilnfile, list BumpList) (BumpList, error) {
	fetchReleasesFromRepo := func(from, to *semver.Version, repository string) []*github.RepositoryRelease {
		owner, repo, err := component.OwnerAndRepoFromGitHubURI(repository)
		if err != nil {
			return nil
		}

		var result []*github.RepositoryRelease

		ops := github.ListOptions{}
		for {
			releases, _, _ := repoService.ListReleases(ctx, owner, repo, &ops)
			if len(releases) == 0 {
				break
			}
			for _, rel := range releases {
				rv, err := semver.NewVersion(strings.TrimPrefix(rel.GetTagName(), "v"))
				if err != nil || rv.LessThan(from) || rv.Equal(from) || rv.GreaterThan(to) {
					continue
				}
				result = append(result, rel)
			}
			ops.Page++
		}

		return result
	}

	fetchReleasesForBump := func(bump BoshReleaseBump) BoshReleaseBump {
		spec := kf.Spec(bump.Name)

		to, from, err := bump.toFrom()
		if err != nil {
			return bump
		}

		for _, repository := range spec.GitRepositories {
			releases := fetchReleasesFromRepo(to, from, repository)
			bump.Releases = append(bump.Releases, releases...)
		}
		sort.Sort(sort.Reverse(releasesByIncreasingSemanticVersion(bump.Releases)))
		bump.deduplicateReleasesWithTheSameTagName()

		return bump
	}

	for i, bump := range list {
		list[i] = fetchReleasesForBump(bump)
	}
	return list, nil
}

//counterfeiter:generate -o ./fakes_internal/issue_getter.go --fake-name IssueGetter . issueGetter

type issueGetter interface {
	Get(ctx context.Context, owner string, repo string, number int) (*github.Issue, *github.Response, error)
}

func issuesFromIssueIDs(ctx context.Context, issuesService issueGetter, repoOwner, repoName string, issueIDs []string) ([]*github.Issue, error) {
	var issues []*github.Issue
	for _, id := range issueIDs {
		n, err := strconv.Atoi(id)
		if err != nil {
			return nil, fmt.Errorf("failed to parse issue id %q: %w", id, err)
		}
		issue, response, err := issuesService.Get(ctx, repoOwner, repoName, n)
		if err != nil {
			return nil, err
		}
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to get issue %q: %w", id, err)
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

//counterfeiter:generate -o ./fakes_internal/milestone_lister.go --fake-name MilestoneLister . milestoneLister

type milestoneLister interface {
	ListMilestones(ctx context.Context, owner string, repo string, opts *github.MilestoneListOptions) ([]*github.Milestone, *github.Response, error)
}

func resolveMilestoneNumber(ctx context.Context, issuesService milestoneLister, repoOwner, repoName, milestone string) (string, error) {
	if milestone == "" {
		return "", nil
	}
	_, parseErr := strconv.Atoi(milestone)
	if parseErr == nil {
		return milestone, nil
	}

	queryOptions := &github.MilestoneListOptions{
		State: "all",
	}
	for {
		ms, res, err := issuesService.ListMilestones(ctx, repoOwner, repoName, queryOptions)
		if err != nil {
			return "", err
		}
		if res.Response.StatusCode != http.StatusOK {
			return "", fmt.Errorf("unexpedted status code %d", res.Response.StatusCode)
		}
		if len(ms) == 0 {
			return "", fmt.Errorf("failed to find milestone with title %q", milestone)
		}
		for _, m := range ms {
			if m.GetTitle() == milestone {
				return strconv.Itoa(m.GetNumber()), nil
			}
		}
		queryOptions.Page++
	}
}

//counterfeiter:generate -o ./fakes_internal/issues_by_repo_lister.go --fake-name IssuesByRepoLister . issuesByRepoLister

type issuesByRepoLister interface {
	ListByRepo(ctx context.Context, owner string, repo string, opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error)
}

func fetchIssuesWithLabelAndMilestone(ctx context.Context, issuesService issuesByRepoLister, repoOwner, repoName, milestoneNumber string, labels []string) ([]*github.Issue, error) {
	if milestoneNumber == "" && len(labels) == 0 {
		return nil, nil
	}
	// TODO: handle pagination
	issueList, response, err := issuesService.ListByRepo(ctx, repoOwner, repoName, &github.IssueListByRepoOptions{
		State:     "all",
		Milestone: milestoneNumber,
		Labels:    labels,
	})
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get issues: %w", err)
	}
	return issueList, nil
}

func appendFilterAndSortIssues(issuesA, issuesB []*github.Issue, filterExp string) []*github.Issue {
	isIDEqual := func(a, b *github.Issue) bool {
		return a.GetID() == b.GetID()
	}
	fullList := appendUnique(issuesA, isIDEqual, issuesB...)

	filtered := filterIssuesByTitle(filterExp, fullList)

	sort.Sort(issuesBySemanticTitlePrefix(filtered))

	return filtered
}

func appendUnique(list []*github.Issue, equal func(a, b *github.Issue) bool, additional ...*github.Issue) []*github.Issue {
nextNewIssue:
	for _, newIssue := range additional {
		for _, existingIssue := range list {
			if equal(newIssue, existingIssue) {
				continue nextNewIssue
			}
		}
		list = append(list, newIssue)
	}
	return list
}

func filterIssuesByTitle(exp string, issues []*github.Issue) []*github.Issue {
	if exp == "" {
		return issues
	}
	titleCheck := regexp.MustCompile(exp)
	filtered := issues[:0]
	for _, issue := range issues {
		if !titleCheck.MatchString(issue.GetTitle()) {
			continue
		}
		filtered = append(filtered, issue)
	}
	return filtered
}

// issuesBySemanticTitlePrefix sorts issues by title lexicographically. It handles issues with a prefix like
// \*\*\[(security fix)|(feature)|(feature improvement)|(bug fix)|(none)\]\*\*, differently and sorts them
// in order of importance.
type issuesBySemanticTitlePrefix []*github.Issue

func (list issuesBySemanticTitlePrefix) Len() int { return len(list) }

func (list issuesBySemanticTitlePrefix) Swap(i, j int) { list[i], list[j] = list[j], list[i] }

func (list issuesBySemanticTitlePrefix) Less(i, j int) bool {
	it := list[i].GetTitle()
	jt := list[j].GetTitle()
	iv := issuesTitlePrefixSemanticValue(it)
	jv := issuesTitlePrefixSemanticValue(jt)
	if iv != jv {
		return iv > jv
	}
	return strings.Compare(it, jt) < 0
}

func issuesTitlePrefixSemanticValue(title string) int {
	title = strings.ToLower(title)
	prefixes := []string{
		"**[security fix]**",
		"**[feature]**",
		"**[feature improvement]**",
		"**[bug fix]**",
		"**[none]**",
	}
	for i, v := range prefixes {
		if strings.HasPrefix(title, v) {
			return len(prefixes) - i
		}
	}
	return 0
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
