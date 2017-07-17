// Copyright 2017 Northern.tech AS
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//        http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.
package tenant

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/mendersoftware/go-lib-micro/apiclient"
	"github.com/pkg/errors"
)

const (
	// devices endpoint
	UriBase         = "/api/internal/v1/tenantadm"
	GetTenantsUri   = UriBase + "/tenants"
	UsersUri        = UriBase + "/users"
	TenantsUsersUri = UriBase + "/tenants/:tid/users/:uid"
	// default request timeout, 10s
	defaultReqTimeout = time.Duration(10) * time.Second
)

var (
	ErrDuplicateUser = errors.New("user with the same name already exists")
	ErrUserNotFound  = errors.New("user not found")
)

// ClientConfig conveys client configuration
type Config struct {
	// tenantadm  service address
	TenantAdmAddr string
	// request timeout
	Timeout time.Duration
}

// ClientRunner is an interface of tenantadm api client
type ClientRunner interface {
	GetTenant(ctx context.Context, username string, client apiclient.HttpRunner) (*Tenant, error)
	CreateUser(ctx context.Context, user *User, client apiclient.HttpRunner) error
	UpdateUser(ctx context.Context, tenantId, userId string, u *UserUpdate, client apiclient.HttpRunner) error
	DeleteUser(ctx context.Context, tenantId, clientId string, client apiclient.HttpRunner) error
}

// Client is an opaque implementation of tenantadm api client.
// Implements ClientRunner interface
type Client struct {
	conf Config
}

// Tenant is the tenantadm's api struct
type Tenant struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// User is the tenantadm's api struct
type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	TenantID string `json:"tenant_id"`
}

// UserUpdate is the tenantadm's api struct
type UserUpdate struct {
	Name string
}

func NewClient(conf Config) *Client {
	if conf.Timeout == 0 {
		conf.Timeout = defaultReqTimeout
	}

	return &Client{
		conf: conf,
	}
}

func (c *Client) GetTenant(ctx context.Context, username string, client apiclient.HttpRunner) (*Tenant, error) {
	req, err := http.NewRequest(http.MethodGet,
		JoinURL(c.conf.TenantAdmAddr, GetTenantsUri+"?username="+username),
		nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request to tenantadm")
	}

	ctx, cancel := context.WithTimeout(ctx, c.conf.Timeout)
	defer cancel()

	rsp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, errors.Wrap(err, "GET /tenants request failed")
	}
	defer rsp.Body.Close()

	if rsp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("GET /tenants request failed with unexpected status %v", rsp.StatusCode)
	}

	tenants := []Tenant{}
	if err := json.NewDecoder(rsp.Body).Decode(&tenants); err != nil {
		return nil, errors.Wrap(err, "error parsing GET /tenants response")
	}

	switch len(tenants) {
	case 1:
		return &tenants[0], nil
	case 0:
		return nil, nil
	default:
		return nil, errors.Errorf("got unexpected number of tenants: %v", len(tenants))
	}
}

func (c *Client) CreateUser(ctx context.Context, user *User, client apiclient.HttpRunner) error {
	// prepare request body
	userJson, err := json.Marshal(user)
	if err != nil {
		return errors.Wrap(err, "failed to prepare body for POST /users")
	}

	reader := bytes.NewReader(userJson)

	req, err := http.NewRequest(http.MethodPost,
		JoinURL(c.conf.TenantAdmAddr, UsersUri),
		reader)
	if err != nil {
		return errors.Wrap(err, "failed to create request for POST /users")
	}

	ctx, cancel := context.WithTimeout(ctx, c.conf.Timeout)
	defer cancel()

	// send
	rsp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return errors.Wrap(err, "POST /users request failed")
	}
	defer rsp.Body.Close()

	switch rsp.StatusCode {
	case http.StatusCreated:
		return nil
	case http.StatusUnprocessableEntity:
		return ErrDuplicateUser
	default:
		return errors.Errorf("POST /users request failed with unexpected status %v", rsp.StatusCode)
	}
}

func (c *Client) UpdateUser(ctx context.Context, tenantId, userId string, u *UserUpdate, client apiclient.HttpRunner) error {
	// prepare request body
	json, err := json.Marshal(u)
	if err != nil {
		return errors.Wrap(err, "failed to prepare body for PUT /tenants/:id/users/:id")
	}

	reader := bytes.NewReader(json)

	repl := strings.NewReplacer(":tid", tenantId, ":uid", userId)
	uri := repl.Replace(TenantsUsersUri)

	req, err := http.NewRequest(http.MethodPut,
		JoinURL(c.conf.TenantAdmAddr, uri),
		reader)
	if err != nil {
		return errors.Wrap(err, "failed to create request for PUT /tenants/:id/users/:id")
	}

	ctx, cancel := context.WithTimeout(ctx, c.conf.Timeout)
	defer cancel()

	// send
	rsp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return errors.Wrap(err, "PUT /tenants/:id/users/:id request failed")
	}
	defer rsp.Body.Close()

	switch rsp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusUnprocessableEntity:
		return ErrDuplicateUser
	case http.StatusNotFound:
		return ErrUserNotFound
	default:
		return errors.Errorf("PUT /tenants/:id/users/:id request failed with unexpected status %v", rsp.StatusCode)
	}
}

func (c *Client) DeleteUser(ctx context.Context, tenantId, userId string, client apiclient.HttpRunner) error {

	repl := strings.NewReplacer(":tid", tenantId, ":uid", userId)
	uri := repl.Replace(TenantsUsersUri)

	req, err := http.NewRequest(http.MethodDelete,
		JoinURL(c.conf.TenantAdmAddr, uri), nil)
	if err != nil {
		return errors.Wrapf(err, "failed to create request for DELETE %s", uri)
	}

	ctx, cancel := context.WithTimeout(ctx, c.conf.Timeout)
	defer cancel()

	// send
	rsp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return errors.Wrapf(err, "DELETE %s request failed", uri)
	}
	defer rsp.Body.Close()

	if rsp.StatusCode != http.StatusNoContent {
		return errors.Errorf("DELETE %s request failed with unexpected status %v", uri, rsp.StatusCode)
	}
	return nil
}

func JoinURL(base, url string) string {
	if strings.HasPrefix(url, "/") {
		url = url[1:]
	}
	if !strings.HasSuffix(base, "/") {
		base = base + "/"
	}
	return base + url
}
