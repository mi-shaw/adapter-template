// Copyright 2023 SGNL.ai, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	framework "github.com/sgnl-ai/adapter-framework"
	api_adapter_v1 "github.com/sgnl-ai/adapter-framework/api/adapter/v1"
)

const (
	// SCAFFOLDING #11 - pkg/adapter/datasource.go: Update the set of valid entity types this adapter supports.

	// PagerDuty implementation: added Teams entity type
	// Extendable: add other entity types from PagerDuty
	Teams string = "teams"
)

// Entity contains entity specific information, such as the entity's unique ID attribute and the
// endpoint to query that entity.
type Entity struct {
	// SCAFFOLDING #12 - pkg/adapter/datasource.go: Update Entity fields used to store entity specific information
	// Add or remove fields as needed. This should be used to store entity specific information
	// such as the entity's unique ID attribute name and the endpoint to query that entity.

	// uniqueIDAttrExternalID is the external ID of the entity's uniqueId attribute.
	uniqueIDAttrExternalID string

	// PagerDuty implementation: added a field to store the endpoint URI to reference when querying the entity
	// TO-DO: implement use of this field
	URI string
}

// Datasource directly implements a Client interface to allow querying
// an external datasource.
type Datasource struct {
	Client *http.Client
}

// PagerDuty implementation: udpated DatasourceResponse struct to DatasourceResponseTeams
// Extendable: add other datasource response structs for other entity types
type DatasourceResponseTeams struct {
	// SCAFFOLDING #13  - pkg/adapter/datasource.go: Add or remove fields in the response as necessary. This is used to unmarshal the response from the SoR.

	// SCAFFOLDING #14 - pkg/adapter/datasource.go: Update `objects` with field name in the SoR response that contains the list of objects.
	Objects []map[string]any `json:"teams,omitempty"`
	Cursor  int              `json:"offset,omitempty"`
	More    bool             `json:"more,omitempty"`
}

var (
	// SCAFFOLDING #15 - pkg/adapter/datasource.go: Update the set of valid entity types supported by this adapter. Used for validation.

	// ValidEntityExternalIDs is a map of valid external IDs of entities that can be queried.
	// The map value is the Entity struct which contains the unique ID attribute.

	// PagerDuty implementation: replaced with Teams entity
	// Extendable: add other valid entity types available from PagerDuty
	ValidEntityExternalIDs = map[string]Entity{
		Teams: {
			uniqueIDAttrExternalID: "id",
		},
	}
)

// NewClient returns a Client to query the datasource.
func NewClient(timeout int) Client {
	return &Datasource{
		Client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

func (d *Datasource) GetPage(ctx context.Context, request *Request) (*Response, *framework.Error) {
	var req *http.Request

	// SCAFFOLDING #16 - pkg/adapter/datasource.go: Create the SoR API URL
	// Populate the request with the appropriate path, headers, and query parameters to query the
	// datasource.

	// PagerDuty: append base URL with the entity external ID as the URI
	// TO-DO: confirm the external ID always matches the URI
	url := fmt.Sprintf("%s/%s", request.BaseURL, request.EntityExternalID)

	// PagerDuty: if PageSize is set, add to query
	if request.PageSize > 0 {
		url = fmt.Sprintf("%s?limit=%d", url, request.PageSize)
	}

	// PagerDuty: if Cursor is set, add to query
	if request.Cursor != "" {
		url = fmt.Sprintf("%s&offset=%s", url, request.Cursor)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, &framework.Error{
			Message: "Failed to create HTTP request to datasource.",
			Code:    api_adapter_v1.ErrorCode_ERROR_CODE_INTERNAL,
		}
	}

	// Timeout API calls that take longer than 5 seconds
	apiCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req = req.WithContext(apiCtx)

	// SCAFFOLDING #17 - pkg/adapter/datasource.go: Add any headers required to communicate with the SoR APIs.
	// Add headers to the request, if any.
	// req.Header.Add("Accept", "application/json")

	// PagerDuty: add Headers
	req.Header.Add("Accept", "")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", request.Token)

	res, err := d.Client.Do(req)
	if err != nil {
		return nil, &framework.Error{
			Message: fmt.Sprintf("Failed to send request to datasource: %v.", err),
			Code:    api_adapter_v1.ErrorCode_ERROR_CODE_INTERNAL,
		}
	}

	response := &Response{
		StatusCode:       res.StatusCode,
		RetryAfterHeader: res.Header.Get("Retry-After"),
	}

	if res.StatusCode != http.StatusOK {
		return response, nil
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, &framework.Error{
			Message: "Failed to read response body.",
			Code:    api_adapter_v1.ErrorCode_ERROR_CODE_DATASOURCE_FAILED,
		}
	}
	// SCAFFOLDING #17-1 - pkg/adapter/datasource.go: To add support for multiple entities that require different parsing functions
	// Add code to call different ParseResponse functions for each entity response.

	// PagerDuty: add switch case for Teams entity and others
	// Extendable: add cases for other entity types
	// TO-DO: fix default case
	switch request.EntityExternalID {
	case Teams:
		objects, nextCursor, parseErr := ParseResponseTeams(body)
		if parseErr != nil {
			return nil, parseErr
		}
		response.Objects = objects
		response.NextCursor = nextCursor

		return response, nil
	default:
		objects, nextCursor, parseErr := ParseResponseTeams(body)
		if parseErr != nil {
			return nil, parseErr
		}
		response.Objects = objects
		response.NextCursor = nextCursor

		return response, nil
	}

}

// PagerDuty: updated ParseResponse function to ParseResponseTeams
// Extendable: add parse response functions for other entity types
func ParseResponseTeams(body []byte) (objects []map[string]any, nextCursor string, err *framework.Error) {
	var data *DatasourceResponseTeams

	unmarshalErr := json.Unmarshal(body, &data)
	if unmarshalErr != nil {
		return nil, "", &framework.Error{
			Message: fmt.Sprintf("Failed to unmarshal the datasource response: %v.", unmarshalErr),
			Code:    api_adapter_v1.ErrorCode_ERROR_CODE_INTERNAL,
		}
	}

	// SCAFFOLDING #18 - pkg/adapter/datasource.go: Add response validations.
	// Add necessary validations to check if the response from the datasource is what is expected.

	// SCAFFOLDING #19 - pkg/adapter/datasource.go: Populate next page information (called cursor in SGNL adapters).
	// Populate nextCursor with the cursor returned from the datasource, if present.

	// PagerDuty: if more results available, retrieve offset and increment 1 to give nextCursor
	if data.More {
		cursorIncrement := data.Cursor + 1
		nextCursor = fmt.Sprintf("%d", cursorIncrement)
	}
	return data.Objects, nextCursor, nil
}
