package bc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const ContentTypeJSON = "application/json"
const NoODATAMetadata = "odata.metadata=none"
const DataAccessReadOnly = "ReadOnly"

// This is the "Accept" header value to return JSON without the OData metadata.
// It's semicolon separated. Included in all requests.
var AcceptJSONNoMetadata = strings.Join([]string{ContentTypeJSON, NoODATAMetadata}, ";")

// MakeRequestOptions are the unique options for the http.Request.
type RequestOptions struct {
	Method        string
	EntitySetName string
	RecordID      uuid.UUID
	QueryParams   QueryParams
	Body          any
}

// Validate checks all the fields for invalid combinations or values.
func (r RequestOptions) Validate() error {
	var errs []string

	// Validate method and entity set to be required.
	if r.Method == "" {
		errs = append(errs, fmt.Sprintf("invalid method: %s", r.Method))
	}

	if r.EntitySetName == "" {
		errs = append(errs, fmt.Sprintf("invalid entitysetname: %s", r.Method))
	}

	// If body exist the method cant be get or delete
	if r.Body != nil {
		if r.Method == http.MethodGet || r.Method == http.MethodDelete {
			errs = append(errs, "invalid combination: cannot have body with GET or DELETE method")
		}
	}
	// Cannot have filter query params with anything but GET
	if r.QueryParams != nil && r.QueryParams["$filter"] != "" {
		if r.Method != http.MethodGet {
			errs = append(errs, fmt.Sprintf("invalid combination: cannot have $filter query param with method %s", r.Method))
		}
	}
	if r.Method == http.MethodPatch && r.RecordID == uuid.Nil {
		errs = append(errs, "invalid combination: cannot have method PATCH with no RecordID")
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid requestoptions: [ %s ]", strings.Join(errs, ", "))
	}
	return nil
}

// QueryParams are used to build the http.Request url.
type QueryParams map[string]string

// NewRequest is the base method that creates the http.Request.
// It has the same return as http.RequestWithContext.
func (c *Client) NewRequest(ctx context.Context, opts RequestOptions) (*http.Request, error) {

	// Validate options
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	// Build the full URL string
	newURL := BuildRequestURL(*c.baseURL, opts.EntitySetName, opts.RecordID, opts.QueryParams)

	// Marshall JSON
	var body io.Reader
	if opts.Body != nil {
		b, err := json.Marshal(opts.Body)
		if err != nil {
			return nil, fmt.Errorf("cannot marshal body %s: %w", opts.Body, err)
		}
		body = bytes.NewReader(b)
	}

	// Create Request
	req, err := http.NewRequestWithContext(ctx, opts.Method, newURL.String(), body)
	if err != nil {
		return nil, fmt.Errorf("creating new request: %w", err)
	}

	// Add the Authorization header for each request
	bearerToken, err := getBearerToken(ctx, c.authClient)
	if err != nil {
		return nil, fmt.Errorf("create auth header: %w", err)
	}
	req.Header.Set("Authorization", bearerToken)

	// Add this header so it doesn't return the extra OData fields
	req.Header.Set("Accept", AcceptJSONNoMetadata)

	// Use ReadOnly for GET
	if opts.Method == http.MethodGet {
		req.Header.Set("Data-Access-Intent", DataAccessReadOnly)
	}

	// Use JSON for POST, PUT, PATCH
	if opts.Method == http.MethodPost || opts.Method == http.MethodPut || opts.Method == http.MethodPatch {
		req.Header.Set("Content-Type", ContentTypeJSON)
	}

	// Use If-Match for POST, PUT, PATCH, DELETE
	if opts.Method == http.MethodDelete || opts.Method == http.MethodPut || opts.Method == http.MethodPatch {
		req.Header.Set("If-Match", "*")
	}

	return req, nil

}

// getBearerToken gets the AccessToken and creates a Bearer token.
func getBearerToken(ctx context.Context, tg TokenGetter) (string, error) {
	accessToken, err := tg.GetToken(ctx)
	if err != nil {
		return "", fmt.Errorf("error adding auth header: %w", err)
	}

	return fmt.Sprintf("Bearer %s", accessToken), nil

}

// Do calls Do on the baseClient.
func (c *Client) Do(r *http.Request) (*http.Response, error) {
	return c.baseClient.Do(r)
}
