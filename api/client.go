package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const baseURL = "https://cvedb.shodan.io"

var httpClient = &http.Client{Timeout: 15 * time.Second}

// apiError extracts the "detail" field from an error response body.
// The field can be a string (e.g. 404) or an array of validation errors (422).
func apiError(resp *http.Response) error {
	var wrapper struct {
		Detail json.RawMessage `json:"detail"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Try string first (e.g. "No information available")
	var detail string
	if json.Unmarshal(wrapper.Detail, &detail) == nil && detail != "" {
		return fmt.Errorf("%s (HTTP %d)", detail, resp.StatusCode)
	}

	// Try array of validation errors (422)
	var validationErrors []struct {
		Loc []any  `json:"loc"`
		Msg string `json:"msg"`
	}
	if json.Unmarshal(wrapper.Detail, &validationErrors) == nil && len(validationErrors) > 0 {
		msgs := make([]string, 0, len(validationErrors))
		for _, ve := range validationErrors {
			field := ""
			for _, l := range ve.Loc {
				if s, ok := l.(string); ok {
					field = s
				}
			}
			if field != "" {
				msgs = append(msgs, fmt.Sprintf("%s: %s", field, ve.Msg))
			} else {
				msgs = append(msgs, ve.Msg)
			}
		}
		return fmt.Errorf("%s (HTTP %d)", strings.Join(msgs, "; "), resp.StatusCode)
	}

	return fmt.Errorf("API returned status %d", resp.StatusCode)
}

func FetchCVE(cveID string) (*CVEWithCPEs, error) {
	resp, err := httpClient.Get(fmt.Sprintf("%s/cve/%s", baseURL, url.PathEscape(cveID)))
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, apiError(resp)
	}

	var result CVEWithCPEs
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}
	return &result, nil
}

type SearchCVEsParams struct {
	Product    string
	CPE23      string
	IsKEV      bool
	SortByEPSS bool
	Skip       int
	Limit      int
}

func SearchCVEs(params SearchCVEsParams) (*CVEs, error) {
	u, _ := url.Parse(baseURL + "/cves")
	q := u.Query()
	if params.Product != "" {
		q.Set("product", params.Product)
	}
	if params.CPE23 != "" {
		q.Set("cpe23", params.CPE23)
	}
	if params.IsKEV {
		q.Set("is_kev", "true")
	}
	if params.SortByEPSS {
		q.Set("sort_by_epss", "true")
	}
	if params.Skip > 0 {
		q.Set("skip", strconv.Itoa(params.Skip))
	}
	limit := params.Limit
	if limit == 0 {
		limit = 50
	}
	q.Set("limit", strconv.Itoa(limit))

	u.RawQuery = q.Encode()

	resp, err := httpClient.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, apiError(resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read failed: %w", err)
	}
	if len(body) == 0 {
		return &CVEs{}, nil
	}
	var result CVEs
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}
	return &result, nil
}

func SearchCPEs(product string, skip, limit int) (*CPEs, error) {
	u, _ := url.Parse(baseURL + "/cpes")
	q := u.Query()
	q.Set("product", product)
	if skip > 0 {
		q.Set("skip", strconv.Itoa(skip))
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	u.RawQuery = q.Encode()

	resp, err := httpClient.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, apiError(resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read failed: %w", err)
	}
	if len(body) == 0 {
		return &CPEs{}, nil
	}
	var result CPEs
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}
	return &result, nil
}
