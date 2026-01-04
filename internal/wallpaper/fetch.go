package wallpaper

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"net/url"
)

// SearchParams captures the query configuration for Wallhaven search.
// Adjust these values to tweak query, category, or sorting behavior.
type SearchParams struct {
	Query      string
	Categories string
	Purity     string
	Sorting    string
}

var DefaultSearchParams = SearchParams{
	// Use OR separators so any of these themes can match instead of requiring all.
	Query:      "nature",
	Categories: "100",
	Purity:     "100",
	Sorting:    "random",
}

const wallhavenSearchEndpoint = "https://wallhaven.cc/api/v1/search"

type searchResponse struct {
	Data []struct {
		Path string `json:"path"`
	} `json:"data"`
}

// FetchBackground fetches and decodes a single background image for the requested resolution.
// It returns an error for invalid dimensions, HTTP failures/non-2xx responses, invalid JSON, or image decode errors.
func FetchBackground(width, height int) (image.Image, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("fetch background: invalid target size %dx%d", width, height)
	}

	imageURL, err := fetchImageURL(width, height, DefaultSearchParams)
	if err != nil {
		return nil, err
	}

	return downloadAndDecode(imageURL)
}

// fetchImageURL calls the search API and extracts the image URL from the response.
// It returns an error if the URL cannot be built, the request fails, the status is non-2xx, or no usable data is returned.
func fetchImageURL(width, height int, params SearchParams) (string, error) {
	searchURL, err := buildSearchURL(width, height, params)
	if err != nil {
		return "", err
	}

	resp, err := http.Get(searchURL)
	if err != nil {
		return "", fmt.Errorf("fetch background: search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("fetch background: search request returned http %d", resp.StatusCode)
	}

	var payload searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("fetch background: decode search failed: %w", err)
	}

	if len(payload.Data) == 0 || payload.Data[0].Path == "" {
		return "", fmt.Errorf("fetch background: no usable image for %dx%d", width, height)
	}

	return payload.Data[0].Path, nil
}

// buildSearchURL builds the full Wallhaven search URL including query parameters for resolution and filters.
// It returns an error if the fixed endpoint cannot be parsed as a URL.
func buildSearchURL(width, height int, params SearchParams) (string, error) {
	values := url.Values{}
	values.Set("q", params.Query)
	values.Set("categories", params.Categories)
	values.Set("purity", params.Purity)
	values.Set("resolutions", fmt.Sprintf("%dx%d", width, height))
	values.Set("sorting", params.Sorting)

	endpoint, err := url.Parse(wallhavenSearchEndpoint)
	if err != nil {
		return "", fmt.Errorf("fetch background: invalid search endpoint: %w", err)
	}
	endpoint.RawQuery = values.Encode()
	return endpoint.String(), nil
}

// downloadAndDecode fetches the resource over HTTP and decodes it via image.Decode.
// It returns an error if the request fails, the status is non-2xx, or the image bytes cannot be decoded.
func downloadAndDecode(resource string) (image.Image, error) {
	resp, err := http.Get(resource)
	if err != nil {
		return nil, fmt.Errorf("fetch background: image request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("fetch background: image request returned http %d", resp.StatusCode)
	}

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("fetch background: decode failed: %w", err)
	}
	return img, nil
}
