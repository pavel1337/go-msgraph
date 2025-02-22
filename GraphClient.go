// Package msgraph is a go lang implementation of the Microsoft Graph API
//
// See: https://developer.microsoft.com/en-us/graph/docs/concepts/overview
package msgraph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// GraphClient represents a msgraph API connection instance.
//
// An instance can also be json-unmarshalled an will immediately be initialized, hence a Token will be
// grabbed. If grabbing a token fails the JSON-Unmarshal returns an error.
type GraphClient struct {
	apiCall sync.Mutex // lock it when performing an API-call to synchronize it

	TenantID      string // See https://docs.microsoft.com/en-us/azure/azure-resource-manager/resource-group-create-service-principal-portal#get-tenant-id
	ApplicationID string // See https://docs.microsoft.com/en-us/azure/azure-resource-manager/resource-group-create-service-principal-portal#get-application-id-and-authentication-key
	ClientSecret  string // See https://docs.microsoft.com/en-us/azure/azure-resource-manager/resource-group-create-service-principal-portal#get-application-id-and-authentication-key

	token Token // the current token to be used
}

func (g *GraphClient) String() string {
	var firstPart, lastPart string
	if len(g.ClientSecret) > 4 { // if ClientSecret is not initialized prevent a panic slice out of bounds
		firstPart = g.ClientSecret[0:3]
		lastPart = g.ClientSecret[len(g.ClientSecret)-3:]
	}
	return fmt.Sprintf("GraphClient(TenantID: %v, ApplicationID: %v, ClientSecret: %v...%v, Token validity: [%v - %v])",
		g.TenantID, g.ApplicationID, firstPart, lastPart, g.token.NotBefore, g.token.ExpiresOn)
}

// NewGraphClient creates a new GraphClient instance with the given parameters and grab's a token.
//
// Rerturns an error if the token can not be initialized. This method does not have to be used to create a new GraphClient
func NewGraphClient(tenantID, applicationID, clientSecret string) (*GraphClient, error) {
	g := GraphClient{TenantID: tenantID, ApplicationID: applicationID, ClientSecret: clientSecret}
	g.apiCall.Lock()         // lock because we will refresh the token
	defer g.apiCall.Unlock() // unlock after token refresh
	return &g, g.refreshToken()
}

// refreshToken refreshes the current Token. Grab's a new one and saves it within the GraphClient instance
func (g *GraphClient) refreshToken() error {
	if g.TenantID == "" {
		return fmt.Errorf("Tenant ID is empty")
	}
	resource := fmt.Sprintf("/%v/oauth2/token", g.TenantID)
	data := url.Values{}
	data.Add("grant_type", "client_credentials")
	data.Add("client_id", g.ApplicationID)
	data.Add("client_secret", g.ClientSecret)
	data.Add("resource", BaseURL)

	u, err := url.ParseRequestURI(LoginBaseURL)
	if err != nil {
		return fmt.Errorf("Unable to parse URI: %v", err)
	}

	u.Path = resource
	req, err := http.NewRequest("POST", u.String(), bytes.NewBufferString(data.Encode()))

	if err != nil {
		return fmt.Errorf("HTTP Request Error: %v", err)
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))

	var newToken Token
	err = g.performRequest(req, &newToken) // perform the prepared request
	if err != nil {
		return fmt.Errorf("Error on getting msgraph Token: %v", err)
	}
	g.token = newToken
	return err
}

// makeGETAPICall performs an API-Call to the msgraph API. This func uses sync.Mutex to synchronize all API-calls
func (g *GraphClient) makeGETAPICall(apicall string, getParams url.Values, v interface{}) error {
	g.apiCall.Lock()
	defer g.apiCall.Unlock() // unlock when the func returns
	// Check token
	if g.token.WantsToBeRefreshed() { // Token not valid anymore?
		err := g.refreshToken()
		if err != nil {
			return err
		}
	}

	reqURL, err := url.ParseRequestURI(BaseURL)
	if err != nil {
		return fmt.Errorf("Unable to parse URI %v: %v", BaseURL, err)
	}

	// Add Version to API-Call, the leading slash is always added by the calling func
	reqURL.Path = "/" + APIVersion + apicall

	req, err := http.NewRequest("GET", reqURL.String(), nil)
	if err != nil {
		return fmt.Errorf("HTTP request error: %v", err)
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", g.token.GetAccessToken())

	if getParams == nil { // initialize getParams if it's nil
		getParams = url.Values{}
	}

	// TODO: Improve performance with using $skip & paging instead of retrieving all results with $top
	// TODO: MaxPageSize is currently 999, if there are any time more than 999 entries this will make the program unpredictable... hence start to use paging (!)
	getParams.Add("$top", strconv.Itoa(MaxPageSize))
	req.URL.RawQuery = getParams.Encode() // set query parameters
	return g.performRequest(req, v)
}

// performRequest performs a pre-prepared http.Request and does the proper error-handling for it.
// does a json.Unmarshal into the v interface{} and returns the error of it if everything went well so far.
func (g *GraphClient) performRequest(req *http.Request, v interface{}) error {
	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP response error: %v of http.Request: %v", err, req.URL)
	}
	defer resp.Body.Close() // close body when func returns

	body, err := ioutil.ReadAll(resp.Body) // read body first to append it to the error (if any)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// Hint: this will mostly be the case if the tenant ID can not be found, the Application ID can not be found or the clientSecret is incorrect.
		// The cause will be described in the body, hence we have to return the body too for proper error-analysis
		return fmt.Errorf("StatusCode is not OK: %v. Body: %v ", resp.StatusCode, string(body))
	}

	//fmt.Println("Body: ", string(body))

	if err != nil {
		return fmt.Errorf("HTTP response read error: %v of http.Request: %v", err, req.URL)
	}

	return json.Unmarshal(body, &v) // return the error of the json unmarshal
}

// ListUsers returns a list of all users
//
// Reference: https://developer.microsoft.com/en-us/graph/docs/api-reference/v1.0/api/user_list
func (g *GraphClient) ListUsers() (Users, error) {
	resource := "/users"
	var marsh struct {
		Users Users `json:"value"`
	}
	err := g.makeGETAPICall(resource, nil, &marsh)
	marsh.Users.setGraphClient(g)
	return marsh.Users, err
}

// ListGroups returns a list of all groups
//
// Reference: https://developer.microsoft.com/en-us/graph/docs/api-reference/v1.0/api/group_list
func (g *GraphClient) ListGroups() (Groups, error) {
	resource := "/groups"

	var marsh struct {
		Groups Groups `json:"value"`
	}
	err := g.makeGETAPICall(resource, nil, &marsh)
	marsh.Groups.setGraphClient(g)
	return marsh.Groups, err
}

// GetUser returns the user object associated to the given user identified by either
// the given ID or userPrincipalName
//
// Reference: https://developer.microsoft.com/en-us/graph/docs/api-reference/v1.0/api/user_get
func (g *GraphClient) GetUser(identifier string) (User, error) {
	resource := fmt.Sprintf("/users/%v", identifier)
	user := User{graphClient: g}
	err := g.makeGETAPICall(resource, nil, &user)
	return user, err
}

// GetGroup returns the group object identified by the given groupID.
//
// Reference: https://developer.microsoft.com/en-us/graph/docs/api-reference/v1.0/api/group_get
func (g *GraphClient) GetGroup(groupID string) (Group, error) {
	resource := fmt.Sprintf("/groups/%v", groupID)
	group := Group{graphClient: g}
	err := g.makeGETAPICall(resource, nil, &group)
	return group, err
}

// ListSignIns returns a list of Sign-Ins
//
// Reference: https://developer.microsoft.com/en-us/graph/docs/api-reference/v1.0/api/signin-list
func (g *GraphClient) ListSignIns() (Signins, error) {
	resource := "/auditlogs/signIns"

	var marsh struct {
		Signins Signins `json:"value"`
	}
	err := g.makeGETAPICall(resource, nil, &marsh)
	marsh.Signins.setGraphClient(g)
	return marsh.Signins, err
}


// ListSignInsWithFilter returns a list of Sign-Ins with filter
//
// Reference: https://developer.microsoft.com/en-us/graph/docs/api-reference/v1.0/api/signin-list
func (g *GraphClient) ListSignInsWithFilter(f string) (Signins, error) {
	resource := "/auditlogs/signIns"

 	v := url.Values{}
 	v.Set("$filter", f)

	var marsh struct {
		Signins Signins `json:"value"`
	}
	err := g.makeGETAPICall(resource, v, &marsh)
	marsh.Signins.setGraphClient(g)
	return marsh.Signins, err
}

// UnmarshalJSON implements the json unmarshal to be used by the json-library.
// This method additionally to loading the TenantID, ApplicationID and ClientSecret
// immediately gets a Token from msgraph (hence initialize this GraphAPI instance)
// and returns an error if any of the data provided is incorrect or the token can not be acquired
func (g *GraphClient) UnmarshalJSON(data []byte) error {
	tmp := struct {
		TenantID      string
		ApplicationID string
		ClientSecret  string
	}{}

	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}

	g.TenantID = tmp.TenantID
	if g.TenantID == "" {
		return fmt.Errorf("TenantID is empty")
	}
	g.ApplicationID = tmp.ApplicationID
	if g.ApplicationID == "" {
		return fmt.Errorf("ApplicationID is empty")
	}
	g.ClientSecret = tmp.ClientSecret
	if g.ClientSecret == "" {
		return fmt.Errorf("ClientSecret is empty")
	}

	// get a token and return the error (if any)
	err = g.refreshToken()
	if err != nil {
		return fmt.Errorf("Can't get Token: %v", err)
	}
	return nil
}
