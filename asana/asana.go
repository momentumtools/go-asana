// Package asana is a client for Asana API.
package asana

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/go-querystring/query"
)

const (
	libraryVersion = "0.1"
	userAgent      = "go-asana/" + libraryVersion
	defaultBaseURL = "https://app.asana.com/api/1.0/"
)

var defaultOptFields = map[string][]string{
	"tags":       {"name", "color", "notes"},
	"users":      {"name", "email", "photo"},
	"projects":   {"name", "color", "archived"},
	"workspaces": {"name", "is_organization"},
	"tasks":      {"name", "assignee", "assignee_status", "completed", "parent"},
}

type (
	// Doer interface used for doing http calls.
	// Use it as point of setting Auth header or custom status code error handling.
	Doer interface {
		Do(req *http.Request) (*http.Response, error)
	}

	// DoerFunc implements Doer interface.
	// Allow to transform any appropriate function "f" to Doer instance: DoerFunc(f).
	DoerFunc func(req *http.Request) (resp *http.Response, err error)

	Client struct {
		doer      Doer
		BaseURL   *url.URL
		UserAgent string
	}

	Workspace struct {
		ID           int64  `json:"id,omitempty"`
		GID          string `json:"gid,omitempty"`
		Name         string `json:"name,omitempty"`
		Organization bool   `json:"is_organization,omitempty"`
	}

	User struct {
		ID         int64             `json:"id,omitempty"`
		GID        string            `json:"gid,omitempty"`
		Email      string            `json:"email,omitempty"`
		Name       string            `json:"name,omitempty"`
		Photo      map[string]string `json:"photo,omitempty"`
		Workspaces []Workspace       `json:"workspaces,omitempty"`
	}

	Team struct {
		GID  string `json:"gid,omitempty"`
		Name string `json:"name,omitempty"`
	}

	Project struct {
		ID       int64  `json:"id,omitempty"`
		GID      string `json:"gid,omitempty"`
		Name     string `json:"name,omitempty"`
		Archived bool   `json:"archived,omitempty"`
		Color    string `json:"color,omitempty"`
		Notes    string `json:"notes,omitempty"`
		Team     *Team  `json:"team,omitempty"`
	}

	Task struct {
		ID             int64         `json:"id,omitempty"`
		GID            string        `json:"gid,omitempty"`
		Assignee       *User         `json:"assignee,omitempty"`
		AssigneeStatus string        `json:"assignee_status,omitempty"`
		CreatedAt      time.Time     `json:"created_at,omitempty"`
		CreatedBy      User          `json:"created_by,omitempty"` // Undocumented field, but it can be included.
		Completed      bool          `json:"completed,omitempty"`
		CompletedAt    time.Time     `json:"completed_at,omitempty"`
		CustomFields   []CustomField `json:"custom_fields,omitempty"`
		Name           string        `json:"name,omitempty"`
		Hearts         []Heart       `json:"hearts,omitempty"`
		Notes          string        `json:"notes,omitempty"`
		ParentTask     *Task         `json:"parent,omitempty"`
		Projects       []Project     `json:"projects,omitempty"`
		DueOn          string        `json:"due_on,omitempty"`
		DueAt          string        `json:"due_at,omitempty"`
	}
	// TaskUpdate is used to update a task.
	TaskUpdate struct {
		Notes   *string `json:"notes,omitempty"`
		Hearted *bool   `json:"hearted,omitempty"`
	}

	Story struct {
		ID        int64     `json:"id,omitempty"`
		GID       string    `json:"gid,omitempty"`
		CreatedAt time.Time `json:"created_at,omitempty"`
		CreatedBy User      `json:"created_by,omitempty"`
		Hearts    []Heart   `json:"hearts,omitempty"`
		Text      string    `json:"text,omitempty"`
		Type      string    `json:"type,omitempty"` // E.g., "comment", "system".
	}

	// Heart represents a ♥ action by a user.
	Heart struct {
		ID   int64 `json:"id,omitempty"`
		User User  `json:"user,omitempty"`
	}

	Tag struct {
		ID    int64  `json:"id,omitempty"`
		GID   string `json:"gid,omitempty"`
		Name  string `json:"name,omitempty"`
		Color string `json:"color,omitempty"`
		Notes string `json:"notes,omitempty"`
	}

	CustomField struct {
		GID                     string        `json:"gid,omitempty"`
		ResourceType            string        `json:"resource_type,omitempty"`
		CurrencyCode            string        `json:"currency_code,omitempty"`
		CustomLabel             string        `json:"custom_label,omitempty"`
		CustomLabelPosition     string        `json:"custom_label_position,omitempty"`
		Description             string        `json:"description,omitempty"`
		Enabled                 bool          `json:"enabled,omitempty"`
		EnumOptions             []EnumOptions `json:"enum_options,omitempty"`
		EnumValue               EnumValue     `json:"enum_value,omitempty"`
		Format                  string        `json:"format,omitempty"`
		HasNotificationsEnabled bool          `json:"has_notifications_enabled,omitempty"`
		IsGlobalToWorkspace     bool          `json:"is_global_to_workspace,omitempty"`
		Name                    string        `json:"name,omitempty"`
		NumberValue             float64       `json:"number_value,omitempty"`
		Precision               int           `json:"precision,omitempty"`
		ResourceSubtype         string        `json:"resource_subtype,omitempty"`
		TextValue               string        `json:"text_value,omitempty"`
		Type                    string        `json:"type,omitempty"`
	}

	EnumOptions struct {
		GID          string `json:"gid,omitempty"`
		ResourceType string `json:"resource_type,omitempty"`
		Color        string `json:"color,omitempty"`
		Enabled      bool   `json:"enabled,omitempty"`
		Name         string `json:"name,omitempty"`
	}

	EnumValue struct {
		GID          string `json:"gid,omitempty"`
		ResourceType string `json:"resource_type,omitempty"`
		Color        string `json:"color,omitempty"`
		Enabled      bool   `json:"enabled,omitempty"`
		Name         string `json:"name,omitempty"`
	}

	Filter struct {
		Archived       *bool         `url:"archived,omitempty"`
		Assignee       int64         `url:"assignee,omitempty"`
		AssigneeGID    int64         `url:"assignee,omitempty"`
		Project        int64         `url:"project,omitempty"`
		ProjectGID     string        `url:"project,omitempty"`
		Workspace      int64         `url:"workspace,omitempty"`
		WorkspaceGID   string        `url:"workspace,omitempty"`
		CompletedSince string        `url:"completed_since,omitempty"`
		ModifiedSince  string        `url:"modified_since,omitempty"`
		OptFields      []string      `url:"opt_fields,comma,omitempty"`
		OptExpand      []string      `url:"opt_expand,comma,omitempty"`
		Offset         string        `url:"offset,omitempty"`
		Limit          uint32        `url:"limit,omitempty"`
		Search         *SearchFilter `url:"-"`
	}

	SearchFilter struct {
		ProjectAll      int64  `url:"projects.all,omitempty"`
		SortBy          string `url:"sort_by,omitempty"`
		SortAscending   *bool  `url:"sort_ascending,omitempty"`
		CreatedAtAfter  string `url:"created_at.after,omitempty"`
		CreatedAtBefore string `url:"created_at.before,omitempty"`
		ModifiedAtAfter string `url:"modified_at.after,omitempty"`
	}

	request struct {
		Data interface{} `json:"data,omitempty"`
	}

	Response struct {
		Data     interface{} `json:"data,omitempty"`
		NextPage *NextPage   `json:"next_page,omitempty"`
		Errors   Errors      `json:"errors,omitempty"`
	}

	Error struct {
		Phrase  string `json:"phrase,omitempty"`
		Message string `json:"message,omitempty"`
	}

	Webhook struct {
		ID       int64    `json:"id,omitempty"`
		GID      string   `json:"gid,omitempty"`
		Resource Resource `json:"resource,omitempty"`
		Target   string   `json:"target",omitempty"`
		Active   bool     `json:"active",omitempty`
	}

	Resource struct {
		ID   int64  `json:"id,omitempty"`
		GID  string `json:"gid,omitempty"`
		Name string `json:"name,omitempty"`
	}

	NextPage struct {
		Offset string `json:"offset,omitempty"`
		Path   string `json:"path,omitempty"`
		URI    string `json:"uri,omitempty"`
	}

	// Errors always has at least 1 element when returned.
	Errors []Error

	// HTTP request error
	RequestError struct {
		Body string
		Code int
	}
)

func (re RequestError) Error() string {
	txt := "request error"
	switch re.Code {
	case http.StatusBadRequest:
		txt = "bad request"
	case http.StatusUnauthorized:
		txt = "unauthorized"
	case http.StatusPaymentRequired:
		txt = "payment required"
	case http.StatusForbidden:
		txt = "forbidden"
	case http.StatusNotFound:
		txt = "not found"
	case http.StatusTooManyRequests:
		txt = "too many requests"
	case http.StatusInternalServerError:
		txt = "internal server error"
	}
	return fmt.Sprintf("asana: Error: %s, got HTTP response code %d", txt, re.Code)
}

func (f DoerFunc) Do(req *http.Request) (resp *http.Response, err error) {
	return f(req)
}

func (e Error) Error() string {
	return fmt.Sprintf("%v - %v", e.Message, e.Phrase)
}

func (e Errors) Error() string {
	var sErrs []string
	for _, err := range e {
		sErrs = append(sErrs, err.Error())
	}
	return strings.Join(sErrs, ", ")
}

// NewClient created new asana client with doer.
// If doer is nil then http.DefaultClient used intead.
func NewClient(doer Doer) *Client {
	if doer == nil {
		doer = http.DefaultClient
	}
	baseURL, _ := url.Parse(defaultBaseURL)
	client := &Client{doer: doer, BaseURL: baseURL, UserAgent: userAgent}
	return client
}

func (c *Client) ListWorkspaces(ctx context.Context) ([]Workspace, error) {
	workspaces := new([]Workspace)
	err := c.Request(ctx, "workspaces", nil, workspaces)
	return *workspaces, err
}

func (c *Client) ListUsers(ctx context.Context, opt *Filter) ([]User, error) {
	users := new([]User)
	err := c.Request(ctx, "users", opt, users)
	return *users, err
}

func (c *Client) ListProjects(ctx context.Context, opt *Filter) ([]Project, error) {
	projects := []Project{}
	for {
		page := []Project{}
		next, err := c.request(ctx, "GET", "projects", nil, nil, opt, &page)
		if err != nil {
			return nil, err
		}
		projects = append(projects, page...)
		if next == nil {
			break
		} else {
			newOpt := *opt
			opt = &newOpt
			opt.Offset = next.Offset
		}
	}
	return projects, nil
}

func (c *Client) ListTasks(ctx context.Context, opt *Filter) ([]Task, error) {
	tasks := []Task{}
	for {
		page := []Task{}
		next, err := c.request(ctx, "GET", "tasks", nil, nil, opt, &page)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, page...)
		if next == nil {
			break
		} else {
			newOpt := *opt
			opt = &newOpt
			opt.Offset = next.Offset
		}
	}
	return tasks, nil
}

func (c *Client) SearchTasks(ctx context.Context, workspaceID int64, opt *Filter) ([]Task, error) {
	tasks := []Task{}
	path := fmt.Sprintf("workspaces/%d/tasks/search", workspaceID)
	err := c.Request(ctx, path, opt, &tasks)
	return tasks, err
}

func (c *Client) GetTask(ctx context.Context, id int64, opt *Filter) (Task, error) {
	task := new(Task)
	err := c.Request(ctx, fmt.Sprintf("tasks/%d", id), opt, task)
	return *task, err
}

func (c *Client) GetTaskByGID(ctx context.Context, id string, opt *Filter) (Task, error) {
	task := new(Task)
	err := c.Request(ctx, fmt.Sprintf("tasks/%s", id), opt, task)
	return *task, err
}

// UpdateTask updates a task.
//
// https://asana.com/developers/api-reference/tasks#update
func (c *Client) UpdateTask(ctx context.Context, id int64, tu TaskUpdate, opt *Filter) (Task, error) {
	task := new(Task)
	_, err := c.request(ctx, "PUT", fmt.Sprintf("tasks/%d", id), tu, nil, opt, task)
	return *task, err
}

func (c *Client) UpdateTaskByGID(ctx context.Context, id string, tu TaskUpdate, opt *Filter) (Task, error) {
	task := new(Task)
	_, err := c.request(ctx, "PUT", fmt.Sprintf("tasks/%s", id), tu, nil, opt, task)
	return *task, err
}

// CreateTask creates a task.
//
// https://asana.com/developers/api-reference/tasks#create
func (c *Client) CreateTask(ctx context.Context, fields map[string]string, opts *Filter) (Task, error) {
	task := new(Task)
	_, err := c.request(ctx, "POST", "tasks", nil, toURLValues(fields), opts, task)
	return *task, err
}

func (c *Client) ListProjectTasks(ctx context.Context, projectID int64, opt *Filter) ([]Task, error) {
	tasks := new([]Task)
	err := c.Request(ctx, fmt.Sprintf("projects/%d/tasks", projectID), opt, tasks)
	return *tasks, err
}

func (c *Client) ListTaskStories(ctx context.Context, taskID int64, opt *Filter) ([]Story, error) {
	stories := new([]Story)
	err := c.Request(ctx, fmt.Sprintf("tasks/%d/stories", taskID), opt, stories)
	return *stories, err
}

func (c *Client) ListTags(ctx context.Context, opt *Filter) ([]Tag, error) {
	tags := new([]Tag)
	err := c.Request(ctx, "tags", opt, tags)
	return *tags, err
}

func (c *Client) GetAuthenticatedUser(ctx context.Context, opt *Filter) (User, error) {
	user := new(User)
	err := c.Request(ctx, "users/me", opt, user)
	return *user, err
}

func (c *Client) GetUserByID(ctx context.Context, id int64, opt *Filter) (User, error) {
	user := new(User)
	err := c.Request(ctx, fmt.Sprintf("users/%d", id), opt, user)
	return *user, err
}

func (c *Client) GetWebhooks(ctx context.Context, opt *Filter) ([]Webhook, error) {
	webhooks := []Webhook{}
	for {
		page := []Webhook{}
		next, err := c.request(ctx, "GET", "webhooks", nil, nil, opt, &page)
		if err != nil {
			return nil, err
		}
		webhooks = append(webhooks, page...)
		if next == nil {
			break
		} else {
			newOpt := *opt
			opt = &newOpt
			opt.Offset = next.Offset
		}
	}
	return webhooks, nil
}

func (c *Client) GetWebhook(ctx context.Context, id int64) (Webhook, error) {
	webhook := new(Webhook)
	err := c.Request(ctx, fmt.Sprintf("webhooks/%d", id), nil, &webhook)
	return *webhook, err
}

func (c *Client) GetWebhookByGID(ctx context.Context, id string) (Webhook, error) {
	webhook := new(Webhook)
	err := c.Request(ctx, fmt.Sprintf("webhooks/%s", id), nil, &webhook)
	return *webhook, err
}

func (c *Client) CreateWebhook(ctx context.Context, id int64, target string) (Webhook, error) {
	webhook := new(Webhook)
	p := url.Values{
		"resource": []string{fmt.Sprintf("%d", id)},
		"target":   []string{target},
	}
	_, err := c.request(ctx, "POST", "webhooks", nil, p, nil, &webhook)
	return *webhook, err
}

func (c *Client) CreateWebhookWithGID(ctx context.Context, id string, target string) (Webhook, error) {
	webhook := new(Webhook)
	p := url.Values{
		"resource": []string{id},
		"target":   []string{target},
	}
	_, err := c.request(ctx, "POST", "webhooks", nil, p, nil, &webhook)
	return *webhook, err
}

func (c *Client) DeleteWebhook(ctx context.Context, id int64) error {
	var resp interface{} // Empty response
	_, err := c.request(ctx, "DELETE", fmt.Sprintf("webhooks/%d", id), nil, nil, nil, &resp)
	return err
}

func (c *Client) DeleteWebhookByGID(ctx context.Context, id string) error {
	var resp interface{} // Empty response
	_, err := c.request(ctx, "DELETE", fmt.Sprintf("webhooks/%s", id), nil, nil, nil, &resp)
	return err
}

func (c *Client) Request(ctx context.Context, path string, opt *Filter, v interface{}) error {
	_, err := c.request(ctx, "GET", path, nil, nil, opt, v)
	return err
}

// request makes a request to Asana API, using method, at path, sending data or form with opt filter.
// Only data or form could be sent at the same time. If both provided form will be omitted.
// Also it's possible to do request with nil data and form.
// The response is populated into v, and any error is returned.
func (c *Client) request(ctx context.Context, method string, path string, data interface{}, form url.Values, opt *Filter, v interface{}) (*NextPage, error) {
	if opt == nil {
		opt = &Filter{}
	}
	if len(opt.OptFields) == 0 {
		// We should not modify opt provided to Request.
		newOpt := *opt
		opt = &newOpt
		opt.OptFields = defaultOptFields[path]
	}
	urlStr, err := addOptions(path, opt)
	if err != nil {
		return nil, err
	}
	rel, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	u := c.BaseURL.ResolveReference(rel)
	var body io.Reader
	if data != nil {
		b, err := json.Marshal(request{Data: data})
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(b)
	} else if form != nil {
		body = strings.NewReader(form.Encode())
	}

	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, err
	}

	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	} else if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	req.Header.Set("User-Agent", c.UserAgent)
	resp, err := c.doer.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// See https://asana.com/developers/documentation/getting-started/errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, rerr := ioutil.ReadAll(resp.Body)
		if rerr != nil {
			return nil, fmt.Errorf("asana: Error reading response body: %s", rerr)
		}
		return nil, &RequestError{
			Body: string(body),
			Code: resp.StatusCode,
		}
	}

	res := &Response{Data: v}
	err = json.NewDecoder(resp.Body).Decode(res)
	if len(res.Errors) > 0 {
		return nil, res.Errors
	}
	return res.NextPage, err
}

func addOptions(s string, opt *Filter) (string, error) {
	u, err := url.Parse(s)
	if err != nil {
		return s, err
	}
	qs, err := query.Values(opt)
	if err != nil {
		return s, err
	}

	if opt != nil {
		sqs, err := query.Values(opt.Search)
		if err != nil {
			return s, err
		}
		for k, v := range sqs {
			qs[k] = v
		}
	}

	u.RawQuery = qs.Encode()
	return u.String(), nil
}

func toURLValues(m map[string]string) url.Values {
	values := make(url.Values)
	for k, v := range m {
		values[k] = []string{v}
	}
	return values
}

// Bool returns a pointer to a bool.
func Bool(b bool) *bool {
	return &b
}
