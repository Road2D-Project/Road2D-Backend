package objectStorage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"Road-To-Destination-BE/module/share"
)

const signTimeout = 10 * time.Second

// Client asks Supabase Storage for a signed upload URL.
// It does not read or write the database. The caller picks the bucket and
// object key, then stores whatever it needs after the client finishes uploading.
type Client struct {
	httpClient *http.Client
}

// SignOptions controls the signed upload token.
// Upsert has to be set here. Sending it on the later file PUT has no effect.
type SignOptions struct {
	Upsert bool
}

// SignedUpload is what the caller hands to the uploader.
// Path is the object key inside the bucket, without a leading slash.
type SignedUpload struct {
	URL  string
	Path string
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: signTimeout},
	}
}

// SignUpload returns a URL the holder can PUT a file to.
// objectPath is a key such as "avatar/<userID>/<userID>.png", not a content type.
func (c *Client) SignUpload(ctx context.Context, bucket, objectPath string, opts SignOptions) (SignedUpload, error) {
	baseURL, key, err := supabaseConfig()
	if err != nil {
		return SignedUpload{}, err
	}
	bucket, err = cleanBucket(bucket)
	if err != nil {
		return SignedUpload{}, err
	}
	objectPath, requestPath, err := cleanObjectPath(objectPath)
	if err != nil {
		return SignedUpload{}, err
	}

	endpoint := baseURL + "/storage/v1/object/upload/sign/" + bucket + "/" + requestPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return SignedUpload{}, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("apikey", key)
	if opts.Upsert {
		req.Header.Set("x-upsert", "true")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return SignedUpload{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return SignedUpload{}, fmt.Errorf("%w: status %d", ErrSignFailed, resp.StatusCode)
	}

	var out struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return SignedUpload{}, fmt.Errorf("%w: %v", ErrSignFailed, err)
	}
	signedURL, err := absoluteSignedURL(baseURL, out.URL)
	if err != nil {
		return SignedUpload{}, err
	}
	return SignedUpload{URL: signedURL, Path: objectPath}, nil
}

// PublicObjectURL is the URL a browser can put in an img tag when the bucket is public.
func (c *Client) PublicObjectURL(bucket, objectPath string) (string, error) {
	baseURL, _, err := supabaseConfig()
	if err != nil {
		return "", err
	}
	bucket, err = cleanBucket(bucket)
	if err != nil {
		return "", err
	}
	_, requestPath, err := cleanObjectPath(objectPath)
	if err != nil {
		return "", err
	}
	return baseURL + "/storage/v1/object/public/" + bucket + "/" + requestPath, nil
}

// Delete removes object keys from a bucket. The caller decides which keys, then updates its own database.
func (c *Client) Delete(ctx context.Context, bucket string, objectPaths []string) error {
	if len(objectPaths) == 0 {
		return ErrInvalidObjectPath
	}
	baseURL, key, err := supabaseConfig()
	if err != nil {
		return err
	}
	bucket, err = cleanBucket(bucket)
	if err != nil {
		return err
	}
	prefixes := make([]string, len(objectPaths))
	for i, objectPath := range objectPaths {
		logical, _, err := cleanObjectPath(objectPath)
		if err != nil {
			return err
		}
		prefixes[i] = logical
	}
	body, err := json.Marshal(struct {
		Prefixes []string `json:"prefixes"`
	}{Prefixes: prefixes})
	if err != nil {
		return err
	}

	endpoint := baseURL + "/storage/v1/object/" + bucket
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("apikey", key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%w: status %d", ErrDeleteFailed, resp.StatusCode)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func supabaseConfig() (string, string, error) {
	baseURL := strings.TrimRight(share.GetEnvStringDefault("SUPABASE_URL", ""), "/")
	key := share.GetEnvStringDefault("SUPABASE_SECRET_KEY", "")
	if baseURL == "" || key == "" {
		return "", "", ErrNotConfigured
	}
	return baseURL, key, nil
}

func cleanBucket(bucket string) (string, error) {
	bucket = strings.Trim(bucket, "/")
	if bucket == "" || strings.Contains(bucket, "/") || bucket == "." || bucket == ".." {
		return "", ErrInvalidObjectPath
	}
	return url.PathEscape(bucket), nil
}

// cleanObjectPath returns the key a caller can store and the escaped path
// used in the sign request. Segments stay separated so folders survive escaping.
func cleanObjectPath(objectPath string) (string, string, error) {
	objectPath = strings.Trim(objectPath, "/")
	if objectPath == "" || strings.Contains(objectPath, "\\") || strings.ContainsRune(objectPath, 0) {
		return "", "", ErrInvalidObjectPath
	}
	parts := strings.Split(objectPath, "/")
	escaped := make([]string, len(parts))
	for i, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", "", ErrInvalidObjectPath
		}
		escaped[i] = url.PathEscape(part)
	}
	return objectPath, strings.Join(escaped, "/"), nil
}

func absoluteSignedURL(baseURL, signedPath string) (string, error) {
	if signedPath == "" {
		return "", ErrSignFailed
	}
	if strings.HasPrefix(signedPath, "https://") || strings.HasPrefix(signedPath, "http://") {
		return signedPath, nil
	}
	if !strings.HasPrefix(signedPath, "/") {
		return "", ErrSignFailed
	}
	return baseURL + "/storage/v1" + signedPath, nil
}
