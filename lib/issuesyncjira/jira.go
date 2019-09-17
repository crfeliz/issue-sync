package issuesyncjira

import (
	"errors"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/coreos/issue-sync/lib/issuesyncgithub"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"time"

	"github.com/andygrunwald/go-jira"
	"github.com/coreos/issue-sync/cfg"
	"github.com/coreos/issue-sync/lib/utils"
	"github.com/google/go-github/v28/github"
)

// commentDateFormat is the format used in the headers of JIRA comments.
const commentDateFormat = "15:04 PM, January 2 2006"

// maxJQLIssueLength is the maximum number of GitHub issues we can
// use before we need to stop using JQL and filter issues ourself.
const maxJQLIssueLength = 100

// getErrorBody reads the HTTP response body of a JIRA API response,
// logs it as an error, and returns an error object with the contents
// of the body. If an error occurs during reading, that error is
// instead printed and returned. This function closes the body for
// further reading.
func getErrorBody(log logrus.Entry, res *jira.Response) error {
	defer func() {
		err := res.Body.Close()
		if err != nil {
			log.Error(err)
		}
	}()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Errorf("Error occurred trying to read error body: %v", err)
		return err
	}
	log.Debugf("Error body: %s", body)
	return errors.New(string(body))
}

// Client is a wrapper around the JIRA API clients library we
// use. It allows us to hide implementation details such as backoff
// as well as swap in other implementations, such as for dry run
// or test mocking.
type Client interface {
	getLogger() logrus.Entry
	getFieldMapper() cfg.FieldMapper
	searchIssues(jql string) (interface{}, *jira.Response, error)
	do(method string, url string, body interface{}, out interface{}) (*jira.Response, error)
	getIssue(key string) (*jira.Issue, *jira.Response, error)
	createIssue(issue *jira.Issue) (*jira.Issue, *jira.Response, error)
	updateIssue(issue *jira.Issue) (*jira.Issue, *jira.Response, error)
	addComment(id string, jComment *jira.Comment, jIssue *jira.Issue, ghComment *github.IssueComment, ghUser *github.User) (*jira.Comment, *jira.Response, error)
	getTransitions(issue jira.Issue) ([]jira.Transition, *jira.Response, error)
	applyTransition(issue jira.Issue, transition jira.Transition) (*jira.Response, error)
}

// realJIRAClient is a standard JIRA clients, which actually makes
// of the requests against the JIRA REST API. It is the canonical
// implementation of Client.
type realJIRAClient struct {
	client jira.Client
	log logrus.Entry
	fieldMapper cfg.FieldMapper
}

// dryrunJIRAClient is an implementation of Client which performs all
// GET requests the same as the realJIRAClient, but does not perform any
// unsafe requests which may modify server data, instead printing out the
// actions it is asked to perform without making the request.
type dryrunJIRAClient struct {
	client jira.Client
	log logrus.Entry
	fieldMapper cfg.FieldMapper
}

type TestJiraClient struct {
	handleGetLogger func() logrus.Entry
	handleGetFieldMapper func() cfg.FieldMapper
	handleSearchIssues func(jql string) (interface{}, *jira.Response, error)
	handleDo func(method string, url string, body interface{}, out interface{}) (*jira.Response, error)
	handleGetIssue func(key string) (*jira.Issue, *jira.Response, error)
	handleCreateIssue func(issue *jira.Issue) (*jira.Issue, *jira.Response, error)
	handleUpdateIssue func(issue *jira.Issue) (*jira.Issue, *jira.Response, error)
	handleAddComment func(id string, jComment *jira.Comment, jIssue *jira.Issue, ghComment *github.IssueComment, ghUser *github.User) (*jira.Comment, *jira.Response, error)
	handleGetTransitions func(issue jira.Issue) ([]jira.Transition, *jira.Response, error)
	handleApplyTransition func(issue jira.Issue, transition jira.Transition) (*jira.Response, error)
}

// Test Client

func (j TestJiraClient) getLogger() logrus.Entry {
	return j.handleGetLogger()
}

func (j TestJiraClient) getFieldMapper() cfg.FieldMapper {
	return j.handleGetFieldMapper()
}

func (j TestJiraClient) getTransitions(issue jira.Issue) ([]jira.Transition, *jira.Response, error) {
	return j.handleGetTransitions(issue)
}

func (j TestJiraClient) applyTransition(issue jira.Issue, transition jira.Transition) (*jira.Response, error) {
	return j.handleApplyTransition(issue, transition)
}

func (j TestJiraClient) searchIssues(jql string) (interface{}, *jira.Response, error) {
	return j.handleSearchIssues(jql)
}

func (j TestJiraClient) getIssue(key string) (*jira.Issue, *jira.Response, error) {
	return j.handleGetIssue(key)
}

func (j TestJiraClient) createIssue(issue *jira.Issue) (*jira.Issue, *jira.Response, error) {
	return j.handleCreateIssue(issue)
}

func (j TestJiraClient) updateIssue(issue *jira.Issue) (*jira.Issue, *jira.Response, error) {
	return j.handleUpdateIssue(issue)
}

func (j TestJiraClient) addComment(id string, jComment *jira.Comment, jIssue *jira.Issue, ghComment *github.IssueComment, ghUser *github.User) (*jira.Comment, *jira.Response, error) {
	return j.handleAddComment(id, jComment, jIssue, ghComment, ghUser)
}

func (j TestJiraClient) do(method string, url string, body interface{}, out interface{}) (*jira.Response, error) {
	return j.handleDo(method, url, body, out)
}

// Real Client
func (j realJIRAClient) getLogger() logrus.Entry {
	return j.log
}

func (j realJIRAClient) getFieldMapper() cfg.FieldMapper {
	return j.fieldMapper
}

func (j realJIRAClient) getTransitions(issue jira.Issue) ([]jira.Transition, *jira.Response, error) {
	return j.client.Issue.GetTransitions(issue.ID)
}

func (j realJIRAClient) applyTransition(issue jira.Issue, transition jira.Transition) (*jira.Response, error) {
	return j.client.Issue.DoTransition(issue.ID, transition.ID)
}

func (j realJIRAClient) searchIssues(jql string) (interface{}, *jira.Response, error) {
	return j.client.Issue.Search(jql, nil)
}

func (j realJIRAClient) getIssue(key string) (*jira.Issue, *jira.Response, error) {
	return j.client.Issue.Get(key, nil)
}

func (j realJIRAClient) createIssue(issue *jira.Issue) (*jira.Issue, *jira.Response, error) {
	return j.client.Issue.Create(issue)
}

func (j realJIRAClient) updateIssue(issue *jira.Issue) (*jira.Issue, *jira.Response, error) {
	return j.client.Issue.Update(issue)
}

func (j realJIRAClient) addComment(id string, jComment *jira.Comment, jIssue *jira.Issue, ghComment *github.IssueComment, ghUser *github.User) (*jira.Comment, *jira.Response, error) {
	return j.client.Issue.AddComment(id, jComment)
}

func (j realJIRAClient) do(method string, url string, body interface{}, out interface{}) (*jira.Response, error) {
	req, _ := j.client.NewRequest(method, url, body)
	return j.client.Do(req, out)
}

// DRY RUN CLIENT
func (j dryrunJIRAClient) getLogger() logrus.Entry {
	return j.log
}

func (j dryrunJIRAClient) getFieldMapper() cfg.FieldMapper {
	return j.fieldMapper
}

func (j dryrunJIRAClient) getTransitions(issue jira.Issue) ([]jira.Transition, *jira.Response, error) {
	return j.client.Issue.GetTransitions(issue.ID)
}

func (j dryrunJIRAClient) applyTransition(issue jira.Issue, transition jira.Transition) (*jira.Response, error) {
	log := j.log
	log.Info("")
	log.Info("Applying Transition:")
	log.Infof("  Jira Issue ID: %s", issue.ID)
	log.Infof("  Transition Id: %s", transition.ID)
	log.Infof("  Old Status: %s", issue.Fields.Status.Name)
	log.Infof("  New Satus: %s", transition.Name)
	log.Info("")
	return nil, nil
}

func (j dryrunJIRAClient) searchIssues(jql string) (interface{}, *jira.Response, error) {
	return j.client.Issue.Search(jql, nil)
}

func (j dryrunJIRAClient) getIssue(key string) (*jira.Issue, *jira.Response, error) {
	return j.client.Issue.Get(key, nil)
}

func (j dryrunJIRAClient) createIssue(issue *jira.Issue) (*jira.Issue, *jira.Response, error) {
	log := j.log

	fields := issue.Fields

	log.Info("")
	log.Info("Create new JIRA issue:")
	log.Infof("  Summary: %s", fields.Summary)
	log.Infof("  Description: %s", truncate(fields.Description, 50))
	log.Infof("  GitHub ID: %d", utils.GetOrElse(j.fieldMapper.GetFieldValue(issue, cfg.GitHubID))(""))
	log.Infof("  GitHub Number: %d", utils.GetOrElse(j.fieldMapper.GetFieldValue(issue, cfg.GitHubNumber))(-1))
	log.Infof("  GitHub Labels: %s", utils.GetOrElse(j.fieldMapper.GetFieldValue(issue, cfg.GitHubLabels))(""))
	log.Infof("  GitHub Status: %s", utils.GetOrElse(j.fieldMapper.GetFieldValue(issue, cfg.GitHubStatus))(""))
	log.Infof("  GitHub Reporter: %s", utils.GetOrElse(j.fieldMapper.GetFieldValue(issue, cfg.GitHubReporter))(""))
	log.Info("")

	return issue, nil, nil
}

func (j dryrunJIRAClient) updateIssue(issue *jira.Issue) (*jira.Issue, *jira.Response, error) {
	log := j.log

	fields := issue.Fields

	log.Info("")
	log.Infof("Update JIRA issue %s:", issue.Key)
	log.Infof("  Summary: %s", fields.Summary)
	log.Infof("  Description: %s", truncate(fields.Description, 50))
	if labels, err := j.fieldMapper.GetFieldValue(issue, cfg.GitHubLabels); err == nil {
		log.Infof("  Labels: %s", labels)
	}
	if state, err := j.fieldMapper.GetFieldValue(issue, cfg.GitHubStatus); err == nil {
		log.Infof("  State: %s", state)
	}
	log.Info("")

	return issue, nil, nil
}

func (j dryrunJIRAClient) addComment(id string, jComment *jira.Comment, jIssue *jira.Issue, ghComment *github.IssueComment, ghUser *github.User) (*jira.Comment, *jira.Response, error) {
	log := j.log

	body := fmt.Sprintf("Comment (ID %d) from GitHub user %s", ghComment.GetID(), ghUser.GetLogin())
	if ghUser.GetName() != "" {
		body = fmt.Sprintf("%s (%s)", body, ghUser.GetName())
	}
	body = fmt.Sprintf(
		"%s at %s:\n\n%s",
		body,
		ghComment.CreatedAt.Format(commentDateFormat),
		ghComment.GetBody(),
	)

	log.Info("")
	log.Infof("Create comment on JIRA issue %s:", jIssue.Key)
	log.Infof("  GitHub ID: %d", ghComment.GetID())
	if ghUser.GetName() != "" {
		log.Infof("  User: %s (%s)", ghUser.GetLogin(), ghUser.GetName())
	} else {
		log.Infof("  User: %s", ghUser.GetLogin())
	}
	log.Infof("  Posted at: %s", ghComment.CreatedAt.Format(commentDateFormat))
	log.Infof("  Body: %s", truncate(ghComment.GetBody(), 100))
	log.Info("")

	return &jira.Comment{
		Body: body,
	}, nil, nil
}

func (j dryrunJIRAClient) do(method string, url string, body interface{}, out interface{}) (*jira.Response, error) {
	disallowedMethods := map[string]bool{"POST": true, "PUT": true}

	if disallowedMethods[method] {
		return nil, nil
	}
	req, err := j.client.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	return j.client.Do(req, out)
}

// NewClient creates a new Client and configures it with
// the config object provided. The type of clients created depends
// on the configuration; currently, it creates either a standard
// clients, or a dry-run clients.
func NewClient(config *cfg.Config) (Client, error) {
	log := config.GetLogger()

	var httpClient *http.Client
	var err error

	if config.IsBasicAuth() {
		transport := jira.BasicAuthTransport{
			Username: config.GetConfigString("jira-user"),
			Password: config.GetConfigString("jira-pass"),
		}
		httpClient = transport.Client()
	} else {
		httpClient, err = newJIRAHTTPClient(*config)
		if err != nil {
			log.Errorf("Error getting OAuth config: %v", err)
			return dryrunJIRAClient{}, err
		}
	}

	client, err := jira.NewClient(httpClient, config.GetConfigString("jira-uri"))
	if err != nil {
		log.Errorf("Error initializing JIRA clients; check your base URI. Error: %v", err)
		return dryrunJIRAClient{}, err
	}

	log.Debug("JIRA clients initialized")

	err = config.LoadJIRAConfig(*client)

	if err != nil {
		log.Error("Error loading config", err)
		return dryrunJIRAClient{}, err
	}

	var j Client

	if config.IsDryRun() {
		j = dryrunJIRAClient{
			log: config.GetLogger(),
			client: *client,
			fieldMapper: config.GetFieldMapper(),
		}
	} else {
		j = realJIRAClient{
			client: *client,
			log: config.GetLogger(),
			fieldMapper: config.GetFieldMapper(),
		}
	}

	return j, nil
}

func TryApplyTransitionWithStatusName(j Client, issue jira.Issue, statusName string) error {
	log := j.getLogger()


	var currentStatusName string
	if issue.Fields == nil || issue.Fields.Status == nil {
		currentStatusName = ""
	} else {
		currentStatusName = strings.ToLower(issue.Fields.Status.Name)
	}

	targetStatusName := strings.ToLower(statusName)
	if currentStatusName == targetStatusName {
		log.Debug("Issue Status is already in sync")
		return nil
	}

	transitions, res, err := j.getTransitions(issue)

	if err != nil {
		log.Errorf("Error retrieving JIRA transitions: %v", err)
		return getErrorBody(j.getLogger(), res)
	}

	for _, v := range transitions {
		if strings.ToLower(v.To.Name) == targetStatusName {
			log.Info(fmt.Sprintf("Applying transition %s -> %s on issue %s", currentStatusName, v.To.Name, issue.ID))
			_, err = j.applyTransition(issue, v)
			if err != nil {
				log.Errorf("Error applying JIRA transitions: %v", err)
				return getErrorBody(j.getLogger(), res)
			} else {
				return nil
			}
		}
	}
	// TODO: This is where we can decide what to do about invalid transitions
	return errors.New(fmt.Sprintf("No transition from '%s' to '%s' found for issue %s", currentStatusName, statusName, issue.ID))
}

// ListIssues returns a list of JIRA issues on the configured project which
// have GitHub IDs in the provided list. `ids` should be a comma-separated
// list of GitHub IDs.
func ListIssues(j Client, timeout time.Duration, jiraProjectKey string,  githubIdFieldId string,  ghIssueIds []int64) ([]jira.Issue, error) {
	log := j.getLogger()

	ghIssueIdStrs := make([]string, len(ghIssueIds))
	for i, v := range ghIssueIds {
		ghIssueIdStrs[i] = fmt.Sprint(v)
	}

	_, isDefaultFieldMapper := j.getFieldMapper().(cfg.DefaultFieldMapper)
	filterWithJql := isDefaultFieldMapper && len(ghIssueIds) < maxJQLIssueLength

	var jql string
	// If the list of IDs is too long, we get a 414 Request-URI Too Large, so in that case,
	// we'll need to do the filtering ourselves.
	if filterWithJql {
		jql = fmt.Sprintf("project='%s' AND cf[%s] in (%s)",
			jiraProjectKey, githubIdFieldId, strings.Join(ghIssueIdStrs, ","))
	} else {
		jql = fmt.Sprintf("project='%s'", jiraProjectKey)
	}

	ji, res, err := utils.Retry(log, timeout, func() (interface{}, interface{}, error) {
		return j.searchIssues(jql)
	})

	if err != nil {
		log.Errorf("Error retrieving JIRA issues: %v", err)
		return nil, getErrorBody(j.getLogger(), res.(*jira.Response))
	}

	var jiraIssues []jira.Issue
	// simulating a set
	projectKeySet := make(map[string] bool)
	jis, ok := ji.([]jira.Issue)
	for _, v := range jis {
		projectKeySet[v.Fields.Project.Key] = true
	}

	projectKeys := make([]string, 0, len(projectKeySet))
	for k := range projectKeySet {
		projectKeys = append(projectKeys, k)
	}

	for _, v := range jis {
		jiraIssues = append(jiraIssues, v)
	}

	if !ok {
		log.Errorf("Get JIRA issues did not return issues! Got: %v", ji)
		return nil, fmt.Errorf("get JIRA issues failed: expected []jira.Issue; got %T", ji)
	}

	var issues []jira.Issue
	if filterWithJql {
		// The issues were already filtered by our JQL, so use as is
		issues = jiraIssues
	} else {
		// Filter only issues which have a defined GitHub ID in the list of IDs
		for _, v := range jiraIssues {
			if id, err := j.getFieldMapper().GetFieldValue(&v, cfg.GitHubID); err == nil {
				for _, idOpt := range ghIssueIds {
					if id.(int64) == int64(idOpt) {
						issues = append(issues, v)
						break
					}
				}
			}
		}
	}
	return issues, nil
}

// GetIssue returns a single JIRA issue within the configured project
// according to the issue key (e.g. "PROJ-13").
func GetIssue(j Client, timeout time.Duration, key string) (jira.Issue, error) {
	log := j.getLogger()

	i, res, err := utils.Retry(log, timeout, func() (interface{}, interface{}, error) {
		return j.getIssue(key)
	})
	if err != nil {
		log.Errorf("Error retrieving JIRA issue: %v", err)
		return jira.Issue{}, getErrorBody(j.getLogger(), res.(*jira.Response))
	}
	issue, ok := i.(*jira.Issue)
	if !ok {
		log.Errorf("Get JIRA issue did not return issue! Got %v", i)
		return jira.Issue{}, fmt.Errorf("Get JIRA issue failed: expected *jira.Issue; got %T", i)
	}

	return *issue, nil
}

// CreateIssue creates a new JIRA issue according to the fields provided in
// the provided issue object. It returns the created issue, with all the
// fields provided (including e.g. ID and Key).
func CreateIssue(j Client, timeout time.Duration, issue jira.Issue) (jira.Issue, error) {
	log := j.getLogger()

	i, res, err := utils.Retry(log, timeout, func() (interface{}, interface{}, error) {
		return j.createIssue(&issue)
	})
	if err != nil {
		log.Errorf("Error creating JIRA issue: %v", err)
		return jira.Issue{}, getErrorBody(j.getLogger(), res.(*jira.Response))
	}
	is, ok := i.(*jira.Issue)
	if !ok {
		log.Errorf("Create JIRA issue did not return issue! Got: %v", i)
		return jira.Issue{}, fmt.Errorf("Create JIRA issue failed: expected *jira.Issue; got %T", i)
	}

	return *is, nil
}

// UpdateIssue updates a given issue (identified by the Key field of the provided
// issue object) with the fields on the provided issue. It returns the updated
// issue as it exists on JIRA.
func UpdateIssue(j Client, timeout time.Duration, issue jira.Issue) (jira.Issue, error) {
	log := j.getLogger()

	i, res, err := utils.Retry(log, timeout, func() (interface{}, interface{}, error) {
		return j.updateIssue(&issue)
	})
	if err != nil {
		log.Errorf("Error updating JIRA issue %s: %v", issue.Key, err)
		return jira.Issue{}, getErrorBody(j.getLogger(), res.(*jira.Response))
	}
	is, ok := i.(*jira.Issue)
	if !ok {
		log.Errorf("Update JIRA issue did not return issue! Got: %v", i)
		return jira.Issue{}, fmt.Errorf("Update JIRA issue failed: expected *jira.Issue; got %T", i)
	}

	return *is, nil
}

// maxBodyLength is the maximum length of a JIRA comment body, which is currently
// 2^15-1.
const maxBodyLength = 1 << 15

// CreateComment adds a comment to the provided JIRA issue using the fields from
// the provided GitHub comment. It then returns the created comment.
func CreateComment(j Client, timeout time.Duration, jIssue jira.Issue, ghComment github.IssueComment, g issuesyncgithub.Client) (jira.Comment, error) {
	log := j.getLogger()

	user, err := issuesyncgithub.GetUser(g, log, timeout, ghComment.User.GetLogin())
	if err != nil {
		return jira.Comment{}, err
	}

	body := fmt.Sprintf("Comment [(ID %d)|%s]", ghComment.GetID(), ghComment.GetHTMLURL())
	body = fmt.Sprintf("%s from GitHub user [%s|%s]", body, user.GetLogin(), user.GetHTMLURL())
	if user.GetName() != "" {
		body = fmt.Sprintf("%s (%s)", body, user.GetName())
	}
	body = fmt.Sprintf(
		"%s at %s:\n\n%s",
		body,
		ghComment.CreatedAt.Format(commentDateFormat),
		ghComment.GetBody(),
	)

	if len(body) > maxBodyLength {
		body = body[:maxBodyLength]
	}

	jComment := jira.Comment{
		Body: body,
	}

	ghUser, err := issuesyncgithub.GetUser(g, log, timeout, ghComment.User.GetLogin())
	if err != nil {
		return jira.Comment{}, err
	}

	com, res, err := utils.Retry(log, timeout, func() (interface{}, interface{}, error) {
		return j.addComment(jIssue.ID, &jComment, &jIssue, &ghComment, &ghUser)
	})
	if err != nil {
		log.Errorf("Error creating JIRA ghComment on jIssue %s. Error: %v", jIssue.Key, err)
		return jira.Comment{}, getErrorBody(log, res.(*jira.Response))
	}
	co, ok := com.(*jira.Comment)
	if !ok {
		log.Errorf("Create JIRA ghComment did not return ghComment! Got: %v", com)
		return jira.Comment{}, fmt.Errorf("Create JIRA ghComment failed: expected *jira.Comment; got %T", com)
	}
	return *co, nil
}

// UpdateComment updates a comment (identified by the `id` parameter) on a given
// JIRA with a new body from the fields of the given GitHub comment. It returns
// the updated comment.
func UpdateComment(j Client, timeout time.Duration, issue jira.Issue, id string, comment github.IssueComment, g issuesyncgithub.Client) (jira.Comment, error) {
	log := j.getLogger()

	user, err := issuesyncgithub.GetUser(g, log, timeout, comment.User.GetLogin())
	if err != nil {
		return jira.Comment{}, err
	}

	body := fmt.Sprintf("Comment [(ID %d)|%s]", comment.GetID(), comment.GetHTMLURL())
	body = fmt.Sprintf("%s from GitHub user [%s|%s]", body, user.GetLogin(), user.GetHTMLURL())
	if user.GetName() != "" {
		body = fmt.Sprintf("%s (%s)", body, user.GetName())
	}
	body = fmt.Sprintf(
		"%s at %s:\n\n%s",
		body,
		comment.CreatedAt.Format(commentDateFormat),
		comment.GetBody(),
	)

	if len(body) > maxBodyLength {
		body = body[:maxBodyLength]
	}

	// As it is, the JIRA API we're using doesn't have any way to update comments natively.
	// So, we have to build the request ourselves.
	requestBody := struct {
		Body string `json:"body"`
	}{
		Body: body,
	}

	jComment := new(jira.Comment)

	com, res, err := utils.Retry(log, timeout, func() (interface{}, interface{}, error) {
		url := fmt.Sprintf("rest/api/2/issue/%s/comment/%s", issue.Key, id)
		res, err := j.do("PUT", url, requestBody, nil)
		return jComment, res, err
	})
	if err != nil {
		log.Errorf("error updating comment: %v", err)
		return jira.Comment{}, getErrorBody(j.getLogger(), res.(*jira.Response))
	}
	co, ok := com.(*jira.Comment)
	if !ok {
		log.Errorf("update JIRA comment did not return comment! Got: %v", com)
		return jira.Comment{}, fmt.Errorf("update JIRA comment failed: expected *jira.Comment; got %T", com)
	}
	return *co, nil
}

// newlineReplaceRegex is a regex to match both "\r\n" and just "\n" newline styles,
// in order to allow us to escape both sequences cleanly in the output of a dry run.
var newlineReplaceRegex = regexp.MustCompile("\r?\n")

// truncate is a utility function to replace all the newlines in
// the string with the characters "\n", then truncate it to no
// more than 50 characters
func truncate(s string, length int) string {
	if s == "" {
		return "empty"
	}

	s = newlineReplaceRegex.ReplaceAllString(s, "\\n")
	if len(s) <= length {
		return s
	}
	return fmt.Sprintf("%s...", s[0:length])
}

func NewTestClient() TestJiraClient {
	return TestJiraClient {}
}


