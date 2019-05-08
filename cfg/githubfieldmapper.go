package cfg

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/andygrunwald/go-jira"
	"github.com/google/go-github/github"
	"strings"
	"time"
)

type FieldMapper interface {
	MapFields(issue *github.Issue) jira.IssueFields

	GetFieldValue(jIssue *jira.Issue, fieldKey FieldKey) (interface{}, error)

	// getFieldIDs requests the metadata of every issue field in the JIRA
	// project, and saves the IDs of the custom fields used by issue-sync.
	GetFieldIDs(client jira.Client) (map[FieldKey]string, error)
}

type DefaultFieldMapper struct {
	Config *Config
}

type JsonFieldMapper struct {
	Config *Config
}

func (m DefaultFieldMapper) GetFieldValue(jIssue *jira.Issue, fieldKey FieldKey) (interface{}, error) {
	var result interface{} = nil
	var err error = nil

	switch fieldKey {
	case GitHubID:
		fallthrough
	case GitHubNumber:
		result, err = jIssue.Fields.Unknowns.Int(m.Config.GetCompleteFieldKey(fieldKey))
	default:
		result, err = jIssue.Fields.Unknowns.String(m.Config.GetCompleteFieldKey(fieldKey))
	}
	return result, err
}

func (m DefaultFieldMapper) MapFields(issue *github.Issue) jira.IssueFields {
	fields := jira.IssueFields{
		Type: jira.IssueType{
			Name: "Task", // TODO: Determine issue type
		},
		Project:     m.Config.GetProject(),
		Summary:     issue.GetTitle(),
		Description: issue.GetBody(),
		Unknowns:    map[string]interface{}{},
	}

	fields.Unknowns[m.Config.GetCompleteFieldKey(GitHubID)] = issue.GetID()
	fields.Unknowns[m.Config.GetCompleteFieldKey(GitHubNumber)] = issue.GetNumber()
	fields.Unknowns[m.Config.GetCompleteFieldKey(GitHubStatus)] = issue.GetState()
	fields.Unknowns[m.Config.GetCompleteFieldKey(GitHubReporter)] = issue.User.GetLogin()

	strs := make([]string, len(issue.Labels))
	for i, v := range issue.Labels {
		strs[i] = *v.Name
	}
	fields.Unknowns[m.Config.GetCompleteFieldKey(GitHubLabels)] = strings.Join(strs, ",")

	fields.Unknowns[m.Config.GetCompleteFieldKey(LastISUpdate)] = time.Now().Format(DateFormat)

	return fields
}

func (m DefaultFieldMapper) GetFieldIDs(client jira.Client) (map[FieldKey]string, error) {
	m.Config.log.Debug("Collecting field IDs.")
	req, err := client.NewRequest("GET", "/rest/api/2/field", nil)
	if err != nil {
		return map[FieldKey]string{}, err
	}
	jFields := new([]jiraField)

	_, err = client.Do(req, jFields)
	if err != nil {
		return map[FieldKey]string{}, err
	}

	fieldIDs := map[FieldKey]string{}

	for _, field := range *jFields {
		switch field.Name {
		case "GitHub ID":
			fieldIDs[GitHubID] = fmt.Sprint(field.Schema.CustomID)
		case "GitHub Number":
			fieldIDs[GitHubNumber] = fmt.Sprint(field.Schema.CustomID)
		case "GitHub Labels":
			fieldIDs[GitHubLabels] = fmt.Sprint(field.Schema.CustomID)
		case "GitHub Status":
			fieldIDs[GitHubStatus] = fmt.Sprint(field.Schema.CustomID)
		case "GitHub Reporter":
			fieldIDs[GitHubReporter] = fmt.Sprint(field.Schema.CustomID)
		case "Last Issue-Sync Update":
			fieldIDs[LastISUpdate] = fmt.Sprint(field.Schema.CustomID)
		}
	}

	_, ok := fieldIDs[GitHubID]
	if !ok {
		return fieldIDs, errors.New("could not find ID of 'GitHub ID' custom field; check that it is named correctly")
	}

	_, ok = fieldIDs[GitHubNumber]
	if !ok {
		return fieldIDs, errors.New("could not find ID of 'GitHub Number' custom field; check that it is named correctly")
	}

	_, ok = fieldIDs[GitHubLabels]
	if !ok {
		return fieldIDs, errors.New("could not find ID of 'Github Labels' custom field; check that it is named correctly")
	}

	_, ok = fieldIDs[GitHubStatus]
	if !ok {
		return fieldIDs, errors.New("could not find ID of 'Github Status' custom field; check that it is named correctly")
	}

	_, ok = fieldIDs[GitHubReporter]
	if !ok {
		return fieldIDs, errors.New("could not find ID of 'Github Reporter' custom field; check that it is named correctly")
	}

	_, ok = fieldIDs[LastISUpdate]
	if !ok  {
		return fieldIDs, errors.New("could not find ID of 'Last Issue-Sync Update' custom field; check that it is named correctly")
	}

	m.Config.log.Debug("All fields have been checked.")

	return fieldIDs, nil
}

func (m JsonFieldMapper) GetFieldValue(jIssue *jira.Issue, fieldKey FieldKey) (interface{}, error) {

	var result interface{} = nil
	var err error = nil

	jsonGithubData, err := jIssue.Fields.Unknowns.String(m.Config.GetCompleteFieldKey(GitHubData))
	if  err != nil {
		return nil, err
	}

	var parsedJson map[string]interface{}
	err = json.Unmarshal([]byte(jsonGithubData), &parsedJson)
	if  err != nil {
		return nil, err
	}

	switch fieldKey {
	case GitHubID:
		result = parsedJson["githubId"]
	case GitHubNumber:
		result = parsedJson["githubNumber"]
	case GitHubLabels:
		result = parsedJson["githubLabels"]
	case GitHubStatus:
		result = parsedJson["githubStatus"]
	case GitHubReporter:
		result = parsedJson["githubReporter"]
	case LastISUpdate:
		result = parsedJson["lastIssueSyncUpdate"]
	}
	return result, err
}

func (m JsonFieldMapper) MapFields(issue *github.Issue) jira.IssueFields {
	fields := jira.IssueFields{
		Type: jira.IssueType{
			Name: "Task", // TODO: Determine issue type
		},
		Project:     m.Config.GetProject(),
		Summary:     issue.GetTitle(),
		Description: issue.GetBody(),
		Unknowns:    map[string]interface{}{},
	}

	githubLabelString := make([]string, len(issue.Labels))
	for i, v := range issue.Labels {
		githubLabelString[i] = *v.Name
	}

	data := map[string]interface{}{
		"githubId": issue.GetID(),
		"githubNumber": issue.GetNumber(),
		"githubStatus":issue.GetState(),
		"githubReporter": issue.User.GetLogin(),
		"githubLabels": githubLabelString,
		"lastIssueSyncUpdate": time.Now().Format(DateFormat),
	}

	fields.Unknowns[m.Config.GetCompleteFieldKey(GitHubData)], _ = json.Marshal(data)

	return fields
}

func (m JsonFieldMapper) GetFieldIDs(client jira.Client) (map[FieldKey]string, error) {
	m.Config.log.Debug("Collecting field IDs.")
	req, err := client.NewRequest("GET", "/rest/api/2/field", nil)
	if err != nil {
		return map[FieldKey]string{}, err
	}
	jFields := new([]jiraField)

	_, err = client.Do(req, jFields)
	if err != nil {
		return map[FieldKey]string{}, err
	}

	fieldIDs := map[FieldKey]string{}

	for _, field := range *jFields {
		switch field.Name {
		case "GitHub Data":
			fieldIDs[GitHubData] = fmt.Sprint(field.Schema.CustomID)
		}
	}

	_, ok := fieldIDs[GitHubData]
	if !ok {
		return fieldIDs, errors.New("could not find ID of 'GitHub Data' custom field; check that it is named correctly")
	}

	m.Config.log.Debug("All fields have been checked.")

	return fieldIDs, nil
}