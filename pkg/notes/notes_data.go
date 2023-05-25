package notes

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/blang/semver/v4"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/google/go-github/v40/github"
	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/pkg/cargo"
	"github.com/pivotal-cf/kiln/pkg/history"
)

type BOSHReleaseData struct {
	cargo.ComponentLock
	Releases []*github.RepositoryRelease
}

func (cd BOSHReleaseData) HasReleaseNotes() bool {
	for _, r := range cd.Releases {
		if len(strings.TrimSpace(r.GetBody())) > 0 {
			return true
		}
	}
	return false
}

type Data struct {
	Version     semver.Version
	ReleaseDate time.Time

	Issues         []*github.Issue
	Components     []BOSHReleaseData
	Bumps          cargo.BumpList
	TrainstatNotes []string

	Stemcell cargo.Stemcell
}

//func (notes Data) Strings() string {
//	note, _ := notes.WriteVersionNotes()
//	return note.Notes
//}

func (notes Data) WriteVersionNotes() (TileRelease, error) {
	noteTemplate, err := DefaultTemplateFunctions(template.New("")).Parse(DefaultTemplate())
	if err != nil {
		return TileRelease{}, err
	}
	notes.Version.Pre = nil
	var buf bytes.Buffer
	err = noteTemplate.Execute(&buf, notes)
	if err != nil {
		return TileRelease{}, err
	}
	return TileRelease{
		Version: notes.Version.String(),
		Notes:   buf.String(),
	}, nil
}

func (notes Data) HasComponentReleases() bool {
	for _, r := range notes.Components {
		if len(r.Releases) > 0 {
			return true
		}
	}
	return false
}

type IssuesQuery struct {
	IssueIDs       []string `long:"github-issue"           short:"i" description:"a list of issues to include in the release notes; these are deduplicated with the issue results"`
	IssueMilestone string   `long:"github-issue-milestone" short:"m" description:"issue milestone to use, it may be the milestone number or the milestone name"`
	IssueLabels    []string `long:"github-issue-label"     short:"l" description:"issue labels to add to issues query"`
	IssueTitleExp  string   `long:"issues-title-exp"       short:"x" description:"issues with title matching regular expression will be added. Issues must first be fetched with github-issue* flags. The default expression can be disabled by setting an empty string" default:"(?i)^\\*\\*\\[(security fix|feature|feature improvement|bug fix|breaking change)\\]\\*\\*.*$"`
}

type TrainstatQuery struct {
	TrainstatURL string `long:"trainstat-url" short:"tu" description:"trainstat url to fetch the release notes for component bumps" default:"https://trainstat.sc2-04-pcf1-apps.oc.vmware.com"`
}

func TrainstatURL() string {
	f, _ := reflect.ValueOf(TrainstatQuery{}).Type().FieldByName("TrainstatURL")
	return f.Tag.Get("default")
}

func IssueTitleRegex() *regexp.Regexp {
	f, _ := reflect.ValueOf(IssuesQuery{}).Type().FieldByName("IssueTitleExp")
	return regexp.MustCompile(f.Tag.Get("default"))
}

func (q IssuesQuery) Exp() (*regexp.Regexp, error) {
	str := q.IssueTitleExp
	if str == "" {
		f, ok := reflect.TypeOf(q).FieldByName("IssueTitleExp")
		if !ok {
			panic("IssueTitleExp field not on IssuesQuery")
		}
		str = f.Tag.Get("default")
	}
	return regexp.Compile(str)
}

func FetchData(ctx context.Context, repo *git.Repository, client *github.Client, tileRepoOwner, tileRepoName, kilnfilePath, initialRevision, finalRevision string, issuesQuery IssuesQuery, trainstatClient TrainstatNotesFetcher) (Data, error) {
	f, err := newFetchNotesData(repo, tileRepoOwner, tileRepoName, kilnfilePath, initialRevision, finalRevision, client, issuesQuery, trainstatClient)
	if err != nil {
		return Data{}, err
	}
	return f.fetch(ctx)
}

// FetchDataWithoutRepo can be used to generate release notes from tile metadata
func FetchDataWithoutRepo(ctx context.Context, client *github.Client, tileRepoOwner, tileRepoName string, kilnfile cargo.Kilnfile, kilnfileLockInitial, kilnfileLockFinal cargo.KilnfileLock, issuesQuery IssuesQuery) (Data, error) {
	r := fetchNotesData{
		repoOwner:       tileRepoOwner,
		repoName:        tileRepoName,
		issuesQuery:     issuesQuery,
		issuesService:   client.Issues,
		releasesService: client.Repositories,
	}
	data := Data{
		Bumps:    cargo.CalculateBumps(kilnfileLockFinal.Releases, kilnfileLockInitial.Releases),
		Stemcell: kilnfileLockFinal.Stemcell,
	}
	var err error
	data.Issues, data.Bumps, err = r.fetchIssuesAndReleaseNotes(ctx, kilnfile, kilnfile, data.Bumps, issuesQuery)
	if err != nil {
		return Data{}, err
	}

	for _, c := range kilnfileLockFinal.Releases {
		data.Components = append(data.Components, BOSHReleaseData{
			ComponentLock: c,
			Releases:      data.Bumps.ForLock(c).Releases,
		})
	}

	return data, nil
}

func newFetchNotesData(repo *git.Repository, tileRepoOwner string, tileRepoName string, kilnfilePath string, initialRevision string, finalRevision string, client *github.Client, issuesQuery IssuesQuery, trainstatClient TrainstatNotesFetcher) (fetchNotesData, error) {
	if repo == nil {
		return fetchNotesData{}, errors.New("git repository required to generate release notes")
	}

	f := fetchNotesData{
		repoOwner:       tileRepoOwner,
		repoName:        tileRepoName,
		kilnfilePath:    kilnfilePath,
		initialRevision: initialRevision,
		finalRevision:   finalRevision,

		historicKilnfile: history.Kilnfile,
		historicVersion:  history.Version,
		revisionResolver: repo,
		Storer:           repo.Storer,
		repository:       repo,

		issuesQuery:     issuesQuery,
		trainstatClient: trainstatClient,
	}

	if client != nil {
		f.issuesService = client.Issues
		f.releasesService = client.Repositories
	}
	return f, nil
}

type fetchNotesData struct {
	historicKilnfile
	historicVersion
	revisionResolver
	storer.Storer
	repository *git.Repository

	issuesService
	releasesService cargo.RepositoryReleaseLister

	repoOwner, repoName,
	kilnfilePath,
	initialRevision, finalRevision string

	issuesQuery     IssuesQuery
	trainstatClient TrainstatNotesFetcher
}

func (r fetchNotesData) fetch(ctx context.Context) (Data, error) {
	initialKilnfileLock, finalKilnfileLock, finalKilnfile, finalVersion, err := r.fetchHistoricFiles(r.kilnfilePath, r.initialRevision, r.finalRevision)
	if err != nil {
		return Data{}, err
	}

	data := Data{
		Version:  finalVersion,
		Bumps:    cargo.CalculateBumps(finalKilnfileLock.Releases, initialKilnfileLock.Releases),
		Stemcell: finalKilnfileLock.Stemcell,
	}

	wtKilnfile, err := r.kilnfileFromWorktree(r.kilnfilePath)
	if err != nil {
		return Data{}, err
	}

	data.Issues, data.Bumps, err = r.fetchIssuesAndReleaseNotes(ctx, finalKilnfile, wtKilnfile, data.Bumps, r.issuesQuery)
	if err != nil {
		return Data{}, err
	}

	majorMinor := fmt.Sprintf("%d.%d", finalVersion.Major, finalVersion.Minor)
	data.TrainstatNotes, err = r.trainstatClient.FetchTrainstatNotes(ctx, r.issuesQuery.IssueMilestone, majorMinor, finalKilnfile.Slug)
	if err != nil {
		return Data{}, err
	}

	for _, c := range finalKilnfileLock.Releases {
		data.Components = append(data.Components, BOSHReleaseData{
			ComponentLock: c,
			Releases:      data.Bumps.ForLock(c).Releases,
		})
	}

	return data, nil
}

func (r fetchNotesData) kilnfileFromWorktree(kilnfilePath string) (cargo.Kilnfile, error) {
	wt, err := r.repository.Worktree()
	if err != nil {
		return cargo.Kilnfile{}, nil
	}
	worktreeKilnfile, err := wt.Filesystem.Open(kilnfilePath)
	if err != nil {
		return cargo.Kilnfile{}, nil
	}
	defer closeAndIgnoreError(worktreeKilnfile)

	buf, err := io.ReadAll(worktreeKilnfile)
	if err != nil {
		return cargo.Kilnfile{}, err
	}

	var wtKf cargo.Kilnfile
	err = yaml.Unmarshal(buf, &wtKf)
	if err != nil {
		return cargo.Kilnfile{}, err
	}

	return wtKf, nil
}

//counterfeiter:generate -o ./fakes/historic_version.go --fake-name HistoricVersion . historicVersion

type historicVersion func(repo storer.EncodedObjectStorer, commitHash plumbing.Hash, kilnfilePath string) (string, error)

//counterfeiter:generate -o ./fakes/historic_kilnfile.go --fake-name HistoricKilnfile . historicKilnfile

type historicKilnfile func(repo storer.EncodedObjectStorer, commitHash plumbing.Hash, kilnfilePath string) (cargo.Kilnfile, cargo.KilnfileLock, error)

//counterfeiter:generate -o ./fakes/revision_resolver.go --fake-name RevisionResolver . revisionResolver

type revisionResolver interface {
	ResolveRevision(rev plumbing.Revision) (*plumbing.Hash, error)
}

func (r fetchNotesData) fetchHistoricFiles(kilnfilePath, start, end string) (klInitial, klFinal cargo.KilnfileLock, kfFinal cargo.Kilnfile, _ semver.Version, _ error) {
	initialCommitSHA, err := r.ResolveRevision(plumbing.Revision(start))
	if err != nil {
		return klInitial, klFinal, kfFinal, semver.Version{}, fmt.Errorf("failed to resolve inital revision %q: %w", start, err)
	}
	finalCommitSHA, err := r.ResolveRevision(plumbing.Revision(end))
	if err != nil {
		return klInitial, klFinal, kfFinal, semver.Version{}, fmt.Errorf("failed to resolve final revision %q: %w", end, err)
	}

	_, klInitial, err = r.historicKilnfile(r.Storer, *initialCommitSHA, kilnfilePath)
	if err != nil {
		return klInitial, klFinal, kfFinal, semver.Version{}, fmt.Errorf("failed to get kilnfile from initial commit: %w", err)
	}
	kfFinal, klFinal, err = r.historicKilnfile(r.Storer, *finalCommitSHA, kilnfilePath)
	if err != nil {
		return klInitial, klFinal, kfFinal, semver.Version{}, fmt.Errorf("failed to get kilnfile from final commit: %w", err)
	}
	version, err := r.historicVersion(r.Storer, *finalCommitSHA, kilnfilePath)
	if err != nil {
		return klInitial, klFinal, kfFinal, semver.Version{}, fmt.Errorf("failed to get version file from final commit: %w", err)
	}

	v, err := semver.ParseTolerant(version)
	if err != nil {
		return klInitial, klFinal, kfFinal, semver.Version{}, fmt.Errorf("failed to parse version: %w", err)
	}

	return klInitial, klFinal, kfFinal, v, nil
}

//counterfeiter:generate -o ./fakes/releases_service.go --fake-name ReleaseService github.com/pivotal-cf/kiln/pkg/cargo.RepositoryReleaseLister
//counterfeiter:generate -o ./fakes/issues_service.go --fake-name IssuesService . issuesService

type issuesService interface {
	issueGetter
	milestoneLister
	issuesByRepoLister
}

// fetchIssuesAndReleaseNotes is not tested. By not testing we are getting reduced abstraction and improved readability at the
// cost of high level testing. This function therefore must stay as small as possible and rely on type checking and a
// manual test to ensure it continues to behave as expected during refactors.
//
// The function can be tested by generating release notes for a tile with issue ids and a milestone set. The happy path
// test for Execute does not set GithubToken intentionally so this code is not triggered and Execute does not actually
// reach out to GitHub.
func (r fetchNotesData) fetchIssuesAndReleaseNotes(ctx context.Context, finalKF, wtKF cargo.Kilnfile, bumpList cargo.BumpList, issuesQuery IssuesQuery) ([]*github.Issue, cargo.BumpList, error) {
	if r.releasesService == nil || r.issuesService == nil {
		return nil, bumpList, nil
	}
	bumpList, err := cargo.ReleaseNotes(ctx, r.releasesService, setEmptyComponentGitHubRepositoryFromOtherKilnfile(finalKF, wtKF), bumpList)
	if err != nil {
		return nil, nil, err
	}

	milestoneNumber, err := resolveMilestoneNumber(ctx, r.issuesService, r.repoOwner, r.repoName, issuesQuery.IssueMilestone)
	if err != nil {
		return nil, nil, err
	}
	issues, err := fetchIssuesWithLabelAndMilestone(ctx, r.issuesService, r.repoOwner, r.repoName, milestoneNumber, issuesQuery.IssueLabels)
	if err != nil {
		return nil, nil, err
	}
	additionalIssues, err := issuesFromIssueIDs(ctx, r.issuesService, r.repoOwner, r.repoName, issuesQuery.IssueIDs)
	if err != nil {
		return nil, nil, err
	}

	return appendFilterAndSortIssues(issues, additionalIssues, issuesQuery.IssueTitleExp), bumpList, nil
}

//counterfeiter:generate -o ./fakes/issue_getter.go --fake-name IssueGetter . issueGetter

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

//counterfeiter:generate -o ./fakes/milestone_lister.go --fake-name MilestoneLister . milestoneLister

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

//counterfeiter:generate -o ./fakes/issues_by_repo_lister.go --fake-name IssuesByRepoLister . issuesByRepoLister

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

func setEmptyComponentGitHubRepositoryFromOtherKilnfile(k1, k2 cargo.Kilnfile) cargo.Kilnfile {
	for i, r := range k1.Releases {
		if r.GitHubRepository != "" {
			continue
		}
		spec, err := k2.ComponentSpec(r.Name)
		if err != nil {
			continue
		}
		k1.Releases[i].GitHubRepository = spec.GitHubRepository
	}
	return k1
}

func closeAndIgnoreError(c io.Closer) { _ = c.Close() }

type TrainstatNotesFetcher interface {
	FetchTrainstatNotes(ctx context.Context, milestone string, version string, tile string) ([]string, error)
}

type TrainstatClient struct {
	host       string
	httpClient *http.Client
}

func NewTrainstatClient(host string) TrainstatClient {
	if strings.TrimSpace(host) == "" {
		host = TrainstatURL()
	}
	client := &http.Client{
		Timeout: time.Second * 10,
	}
	return TrainstatClient{
		host:       host,
		httpClient: client,
	}
}

func (t *TrainstatClient) FetchTrainstatNotes(ctx context.Context, milestone string, version string, tile string) (notes []string, err error) {
	if !t.tileSupported(tile) {
		return []string{}, nil
	}

	baseURL := fmt.Sprintf("%s/%s", t.host, "api/v1/release_notes")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	q := req.URL.Query()
	q.Set("milestone", milestone)
	q.Set("version", version)
	q.Set("tile", tile)
	req.URL.RawQuery = q.Encode()

	res, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request status code is not ok: got %d", res.StatusCode)
	}
	defer closeAndIgnoreError(res.Body)

	responseData, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(responseData, &notes); err != nil {
		return notes, err
	}
	return
}

func (t *TrainstatClient) tileSupported(tile string) bool {
	switch tile {
	case "elastic-runtime", "p-isolation-segment", "pas-windows":
		return true
	default:
		return false
	}
}
