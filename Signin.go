package msgraph

import (
	"encoding/json"
	"fmt"
	"time"
)

// Signin represents one signIn of ms graph
//
// See: https://developer.microsoft.com/en-us/graph/docs/api-reference/v1.0/api/signin-get
type Signin struct {
	ID                      	string
	UserPrincipalName       	string
	UserDisplayName         	string
	CreatedDateTime         	time.Time
	AppDisplayName          	string
	IpAddress               	string
	ClientAppUsed				string
	ResourceDisplayName			string
	DeviceDetail				DeviceDetail
	Location 					Location

	graphClient *GraphClient // the graphClient that called the group
}

type DeviceDetail struct {
	OperatingSystem		string
	Browser 			string
}

type Location struct {
	City	string
	State	string
	CountryOrRegion	string
	GeoCoordinates GeoCoordinates
}

type GeoCoordinates struct {
	Latitude float64 `json:latitude`
	Longitude float64 `json:longitude`
}


func (g Signin) String() string {
	return fmt.Sprintf("Signin(ID: \"%v\", UserPrincipalName: \"%v\" UserDisplayName: \"%v\", CreatedDateTime: \"%v\", AppDisplayName: \"%v\", IpAddress: \"%v\", ClientAppUsed: \"%v\", ResourceDisplayName: \"%v\"",
		g.ID, g.UserPrincipalName, g.UserDisplayName, g.CreatedDateTime, g.AppDisplayName, g.IpAddress, g.ClientAppUsed, g.ResourceDisplayName, g.graphClient != nil)
}

// setGraphClient sets the graphClient instance in this instance and all child-instances (if any)
func (g *Signin) setGraphClient(gC *GraphClient) {
	g.graphClient = gC
}

// UnmarshalJSON implements the json unmarshal to be used by the json-library
func (g *Signin) UnmarshalJSON(data []byte) error {
	tmp := struct {
		ID                       	string   `json:"id"`
		UserPrincipalName       	string   `json:"userPrincipalName"`
		UserDisplayName         	string   `json:"userDisplayName"`
		CreatedDateTime         	string   `json:"createdDateTime"`
		AppDisplayName          	string   `json:"appDisplayName"`
		IpAddress                	string   `json:"ipAddress"`
		ClientAppUsed				string 	 `json:"clientAppUsed"`
		ResourceDisplayName			string 	 `json:"resourceDisplayName"`
		DeviceDetail				DeviceDetail `json:"deviceDetail"`
		Location					Location	`json:"location"`
		GeoCoordinates					GeoCoordinates	`json:"geoCoordinates"`
	}{}

	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}

	g.ID = tmp.ID
	g.UserPrincipalName = tmp.UserPrincipalName
	g.UserDisplayName = tmp.UserDisplayName
	g.CreatedDateTime, err = time.Parse(time.RFC3339, tmp.CreatedDateTime)
	if err != nil && tmp.CreatedDateTime != "" {
		return fmt.Errorf("Can not parse CreatedDateTime %v with RFC3339: %v", tmp.CreatedDateTime, err)
	}
	g.AppDisplayName = tmp.AppDisplayName
	g.IpAddress = tmp.IpAddress
	g.ClientAppUsed = tmp.ClientAppUsed
	g.ResourceDisplayName = tmp.ResourceDisplayName
	g.DeviceDetail = tmp.DeviceDetail
	g.Location = tmp.Location

	return nil
}
