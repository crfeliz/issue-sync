package issuesyncgithub

import (
	"context"
	"github.com/Sirupsen/logrus"
	"github.com/coreos/issue-sync/cfg"
	"github.com/google/go-github/github"
	"testing"
	"time"
)


func TestListIssues(t *testing.T) {

	client := NewTestClient()

	log := *cfg.NewLogger("test", "debug")

	client.handleGetLogger = func() logrus.Entry {
		return log
	}

	client.handleGetRepository = func(ctx context.Context, owner string, repo string) (repository *github.Repository, response *github.Response, e error) {
		hasProject := true
		return &github.Repository{ HasProjects: &hasProject}, nil, nil
	}

	client.handleListByRepo = func(ctx context.Context, owner string, repo string, page int, since time.Time) (issues []*github.Issue, response *github.Response, e error) {
		issues = make([]*github.Issue, 3)
		for i := 0; i < len(issues); i++ {
			issues[i] = &github.Issue{}
		}
		return issues, &github.Response{LastPage:3}, nil
	}

	client.handleListIssueEvents = func(ctx context.Context, owner, repo string, number int, page int) (events []*github.IssueEvent, response *github.Response, e error) {
		events = make([]*github.IssueEvent, 3)
		for i := 0; i < len(events); i++ {
			events[i] = &github.IssueEvent{}
		}
		return events, &github.Response{LastPage:1}, nil
	}

	issues, _ := ListIssues(client, 10, "", "", time.Now())


	if len(issues) != 9 {
		t.Fatalf("Expected len(issues) = 9; Got len(issues) = %d", len(issues))
	}
}
