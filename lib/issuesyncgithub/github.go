package issuesyncgithub

import (
	"context"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/coreos/issue-sync/lib/models"
	"github.com/coreos/issue-sync/lib/utils"
	"time"

	"github.com/coreos/issue-sync/cfg"
	"github.com/google/go-github/v28/github"
	"golang.org/x/oauth2"
)

// Client is a wrapper around the GitHub API Client library we
// use. It allows us to swap in other implementations, such as a dry run
// clients, or mock clients for testing.
type Client interface {
	getLogger() logrus.Entry
	listIssueEvents(ctx context.Context, owner, repo string, number int, page int) ([]*github.IssueEvent, *github.Response, error)
	listByRepo(ctx context.Context, owner string, repo string, page int, since time.Time) ([]*github.Issue, *github.Response, error)
	getRepository(ctx context.Context, owner string, repo string) (*github.Repository, *github.Response, error)
	listComments(ctx context.Context, owner string, repo string, number int) ([]*github.IssueComment, *github.Response, error)
	getUser(ctx context.Context, user string) (*github.User, *github.Response, error)
	getRateLimits(ctx context.Context) (*github.RateLimits, *github.Response, error)
}

// realGHClient is a standard GitHub clients, that actually makes all of the
// requests against the GitHub REST API. It is the canonical implementation
// of Client.
type realGHClient struct {
	client github.Client
	log logrus.Entry
}

func (g realGHClient) getLogger() logrus.Entry {
	return g.log
}

func (g realGHClient) listIssueEvents(ctx context.Context, owner, repo string, number int, page int)  ([]*github.IssueEvent, *github.Response, error) {
	return g.client.Issues.ListIssueEvents(ctx, owner, repo, number, &github.ListOptions{
		Page:    page,
		PerPage: 100,
	})
}

func (g realGHClient) getRepository(ctx context.Context, owner string, repo string) (*github.Repository, *github.Response, error) {
	return g.client.Repositories.Get(ctx, owner, repo)
}

func (g realGHClient) listByRepo(ctx context.Context, owner string, repo string, page int, since time.Time) ([]*github.Issue, *github.Response, error) {
	return g.client.Issues.ListByRepo(ctx, owner, repo, &github.IssueListByRepoOptions{
		Since:     since,
		State:     "all",
		Sort:      "created",
		Direction: "asc",
		ListOptions: github.ListOptions{
			Page:    page,
			PerPage: 100,
		},
	})
}

func (g realGHClient) listComments(ctx context.Context, owner string, repo string, number int) ([]*github.IssueComment, *github.Response, error) {
	return g.client.Issues.ListComments(ctx, owner, repo, number, &github.IssueListCommentsOptions{
		Sort:      "created",
		Direction: "asc",
	})
}

func (g realGHClient) getUser(ctx context.Context, user string) (*github.User, *github.Response, error) {
	return g.client.Users.Get(context.Background(), user)
}

func (g realGHClient) getRateLimits(ctx context.Context) (*github.RateLimits, *github.Response, error) {
	return g.client.RateLimits(ctx)
}

type TestGHClient struct {
	handleGetLogger func() logrus.Entry
	handleListIssueEvents func(ctx context.Context, owner, repo string, number int, page int) ([]*github.IssueEvent, *github.Response, error)
	handleListByRepo func(ctx context.Context, owner string, repo string, page int, since time.Time) ([]*github.Issue, *github.Response, error)
	handleGetRepository func(ctx context.Context, owner string, repo string) (*github.Repository, *github.Response, error)
	handleListComments func(ctx context.Context, owner string, repo string, number int) ([]*github.IssueComment, *github.Response, error)
	handleGetUser func(ctx context.Context, user string) (*github.User, *github.Response, error)
	handleGetRateLimits func(ctx context.Context) (*github.RateLimits, *github.Response, error)
}

func (g TestGHClient) getLogger() logrus.Entry {
	return g.handleGetLogger()
}

func (g TestGHClient) listIssueEvents(ctx context.Context, owner, repo string, number int, page int) ([]*github.IssueEvent, *github.Response, error) {
	return g.handleListIssueEvents(ctx, owner, repo, number, page)
}

func (g TestGHClient) getRepository(ctx context.Context, owner string, repo string) (*github.Repository, *github.Response, error) {
	return g.handleGetRepository(ctx, owner, repo)
}

func (g TestGHClient) listByRepo(ctx context.Context, owner string, repo string, page int, since time.Time) ([]*github.Issue, *github.Response, error) {
	return g.handleListByRepo(ctx, owner, repo, page, since)
}

func (g TestGHClient) listComments(ctx context.Context, owner string, repo string, number int) ([]*github.IssueComment, *github.Response, error) {
	return g.handleListComments(ctx, owner, repo, number)
}

func (g TestGHClient) getUser(ctx context.Context, user string) (*github.User, *github.Response, error) {
	return g.handleGetUser(ctx, user)
}

func (g TestGHClient) getRateLimits(ctx context.Context) (*github.RateLimits, *github.Response, error) {
	return g.getRateLimits(ctx)
}

func getCurrentProjectCardAndCommitIds(g Client, timeout time.Duration, user string, repoName string, issue *github.Issue) (*github.ProjectCard, []string, error) {
	log := g.getLogger()
	ctx := context.Background()
	pages := 1

	var currentProjectCard *github.ProjectCard

	var commitIds []string

	// search for the current project card
	for page := 1; page <= pages; page++ {
		is, res, err := utils.Retry(log, timeout, func() (interface{}, interface{}, error) {
			return g.listIssueEvents(ctx, user, repoName, issue.GetNumber(), page)
		})
		if err != nil {
			return nil, nil, err
		}
		issueEventPointers, ok := is.([]*github.IssueEvent)
		if !ok {
			log.Errorf("get GitHub issue events did not return issue events! Got: %v", is)
			return nil, nil, fmt.Errorf("get GitHub issues events failed: expected []*github.IssueEvent; got %T", is)
		}

		for _, v := range issueEventPointers {
			if v.ProjectCard != nil {
				if v.GetEvent() == "added_to_project" || v.GetEvent() == "moved_columns_in_project" {
					currentProjectCard = v.ProjectCard
				} else if *v.Event == "removed_from_project" {
					currentProjectCard = nil
				}
			}

			if v.CommitID != nil {
				commitIds = append(commitIds, *v.CommitID)
			}
		}
		pages = res.(*github.Response).LastPage
	}

	log.Debugf("Found current Project Card for issue #%d", issue.GetNumber())

	return currentProjectCard, commitIds, nil
}

// ListIssues returns the list of GitHub issues since the last run of the tool.
func ListIssues(g Client, timeout time.Duration, user string, repoName string, since time.Time) ([]models.ExtendedGithubIssue, error) {
	log := g.getLogger()
	ctx := context.Background()
	repo, _, _ := g.getRepository(ctx, user, repoName)

	// Set it so that it will run the loop once, and it'll be updated in the loop.
	pages := 1
	var issues []models.ExtendedGithubIssue

	for page := 1; page <= pages; page++ {
		is, res, err := utils.Retry(log, timeout, func() (interface{}, interface{}, error) {
			return g.listByRepo(ctx, user, repoName, page, since)
		})
		if err != nil {
			return nil, err
		}
		issuePointers, ok := is.([]*github.Issue)
		if !ok {
			log.Errorf("get GitHub issues did not return issues! Got: %v", is)
			return nil, fmt.Errorf("get GitHub issues failed: expected []*github.Issue; got %T", is)
		}

		var issuePage []models.ExtendedGithubIssue
		for _, v := range issuePointers {
			// If PullRequestLinks is not nil, it's a Pull Request
			if v.PullRequestLinks == nil {

				var currentProjectCard *github.ProjectCard
				var commitIds []string
				if repo.GetHasProjects() {
					currentProjectCard, commitIds, _ = getCurrentProjectCardAndCommitIds(g, timeout, user, repoName, v)
				}

				issuePage = append(issuePage, models.ExtendedGithubIssue{Issue: *v, ProjectCard: currentProjectCard, CommitIds: commitIds})
			}
		}

		pages = res.(*github.Response).LastPage
		issues = append(issues, issuePage...)
	}

	log.Debug("Collected all GitHub issues")

	return issues, nil
}

// ListComments returns the list of all comments on a GitHub issue in
// ascending order of creation.
func ListComments(g Client, timeout time.Duration, user string, repoName string, issue github.Issue) ([]*github.IssueComment, error) {
	log := g.getLogger()
	ctx := context.Background()
	c, _, err := utils.Retry(log, timeout, func() (interface{}, interface{}, error) {
		return g.listComments(ctx, user, repoName, issue.GetNumber())
	})
	if err != nil {
		log.Errorf("error retrieving GitHub comments for issue #%d. Error: %v.", issue.GetNumber(), err)
		return nil, err
	}
	comments, ok := c.([]*github.IssueComment)
	if !ok {
		log.Errorf("eet GitHub comments did not return comments! Got: %v", c)
		return nil, fmt.Errorf("get GitHub comments failed: expected []*github.IssueComment; got %T", c)
	}

	return comments, nil
}

// GetUser returns a GitHub user from its login.
func GetUser(g Client, log logrus.Entry, timeout time.Duration, userName string) (github.User, error) {
	u, _, err := utils.Retry(log, timeout, func() (interface{}, interface{}, error) {
		return g.getUser(context.Background(), userName)
	})

	if err != nil {
		log.Errorf("error retrieving GitHub user %s. Error: %v", userName, err)
	}

	user, ok := u.(*github.User)
	if !ok {
		log.Errorf("eet GitHub user did not return user! Got: %v", u)
		return github.User{}, fmt.Errorf("get GitHub user failed: expected *github.User; got %T", u)
	}

	return *user, nil
}

// NewClient creates a Client and returns it; which
// implementation it uses depends on the configuration of this
// run. For example, a dry-run clients may be created which does
// not make any requests that would change anything on the server,
// but instead simply prints out the actions that it's asked to take.
func NewClient(config cfg.Config) (Client, error) {
	var ret Client

	log := config.GetLogger()

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: config.GetConfigString("github-token")},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	ret = realGHClient{
		client: *client,
		log: log,
	}

	// Make a request so we can check that we can connect fine.
	_, _, err := ret.getRateLimits(context.Background())
	if err != nil {
		return realGHClient{}, err
	}
	log.Debug("Successfully connected to GitHub.")

	return ret, nil
}

func NewTestClient() TestGHClient {
	return TestGHClient {}
}

