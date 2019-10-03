package issuesyncjira

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/andygrunwald/go-jira"
	"github.com/coreos/issue-sync/cfg"
	"testing"
)

func TestListIssues(t *testing.T) {

	testProjectKey := "TPK"

	testFieldMapper := cfg.TestFieldMapper{}

	testFieldMapper.HandleGetFieldValue = func(jIssue *jira.Issue, fieldKey cfg.FieldKey) (i interface{}, e error) {
		return int64(1), nil
	}


	client := NewTestClient()

	log := *cfg.NewLogger("test", "debug")

	client.handleGetLogger = func() logrus.Entry {
		return log
	}

	client.handleGetFieldMapper = func() cfg.FieldMapper {
		return testFieldMapper
	}

	client.handleSearchIssues = func(jql string) (i interface{}, response *jira.Response, e error) {
		issues := make([]jira.Issue, 3)
		for i := 0; i < len(issues); i++ {
			issues[i] = jira.Issue{Fields: &jira.IssueFields{Project:jira.Project{Key: testProjectKey}}}
		}
		return issues, &jira.Response{}, nil
	}

	issues, _ := ListIssues(client, 10, testProjectKey, "", []int64 { 1, 2, 3, 4 })


	if len(issues) != 3 {
		t.Fatalf("Expected len(issues) = 3; Got len(issues) = %d", len(issues))
	}
}

func TestTryApplyTransitionWithName(t *testing.T) {
	testProjectKey := "TPK"
	issue := jira.Issue{Fields: &jira.IssueFields{Project:jira.Project{Key: testProjectKey}}}
	client := NewTestClient()
	log := *cfg.NewLogger("test", "debug")

	client.handleGetLogger = func() logrus.Entry {
		return log
	}

	client.handleGetTransitions = func(issue jira.Issue) (transitions []jira.Transition, response *jira.Response, e error) {
		transitions = make([]jira.Transition, 3)
		for i := 0; i < len(transitions); i++ {
			transitions[i] = jira.Transition{To: jira.Status{Name:fmt.Sprintf("Transition_%d", i)}}
		}
		return transitions, &jira.Response{}, nil
	}

	client.handleApplyTransition = func(issue jira.Issue, transition jira.Transition) (response *jira.Response, e error) {
		return &jira.Response{}, nil
	}

	err := TryApplyTransitionWithStatusName(client, issue, "Transition_2")

	if err != nil {
		t.Fatalf("TryApplyTransitionWithStatusName failed with error: %s", err.Error())
	}
}
