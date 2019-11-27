package msgraph

import (
	"strings"
)

// Signins represents multiple Signin-instances.
type Signins []Signin

func (g Signins) String() string {
	var signins = make([]string, len(g))
	for i, s := range g {
		signins[i] = s.String()
	}
	return "Signins(" + strings.Join(signins, " \n ") + ")"
}

// setGraphClient sets the GraphClient within that particular instance. Hence it's directly created by GraphClient
func (g Signins) setGraphClient(gC *GraphClient) Signins {
	for i := range g {
		g[i].setGraphClient(gC)
	}
	return g
}
