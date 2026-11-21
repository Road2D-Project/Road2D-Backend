package objectStorage

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

const (
	AvatarNamespace  = "avatar"
	TripNamespace    = "trip"
	uploadLifetime   = 15 * time.Minute
	downloadLifetime = time.Hour
)

// Client stores all media in one private R2 bucket. Namespaces become key prefixes.
type Client struct {
	s3        *s3.Client
	bucket    string
	policy    MediaPolicy
	configErr error
}

type SignOptions struct {
	ContentType string
	SizeBytes   int64
}

// Headers must be sent with the raw file PUT. Browsers set Content-Length themselves.
type SignedUpload struct {
	URL       string            `json:"url"`
	Path      string            `json:"path"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expiresAt"`
}

type ObjectInfo struct {
	ContentType string
	SizeBytes   int64
}

func NewClient() *Client {
	policy, err := LoadMediaPolicy()
	c := &Client{bucket: strings.TrimSpace(os.Getenv("R2_BUCKET_NAME")), policy: policy, configErr: err}
	accountID := strings.TrimSpace(os.Getenv("R2_ACCOUNT_ID"))
	accessKey := envCredential("R2_ACCESS_KEY_ID", "R2_ACCESS_TOKEN")
	secretKey := envCredential("R2_SECRET_ACCESS_KEY", "R2_SECRET_ACCESS_TOKEN")
	if c.configErr != nil {
		return c
	}
	if c.bucket == "" || accountID == "" || accessKey == "" || secretKey == "" {
		c.configErr = ErrNotConfigured
		return c
	}
	endpoint := strings.TrimSpace(os.Getenv("R2_ENDPOINT"))
	if endpoint == "" {
		endpoint = "https://" + accountID + ".r2.cloudflarestorage.com"
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		c.configErr = ErrNotConfigured
		return c
	}
	c.s3 = s3.New(s3.Options{
		Region: "auto", BaseEndpoint: aws.String(strings.TrimRight(endpoint, "/")),
		Credentials:                credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		HTTPClient:                 &http.Client{Timeout: 10 * time.Second},
		UsePathStyle:               true,
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})
	return c
}

func envCredential(name, legacy string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return strings.TrimSpace(os.Getenv(legacy))
}

func (c *Client) SignUpload(ctx context.Context, namespace, objectPath string, opts SignOptions) (SignedUpload, error) {
	key, err := c.objectKey(namespace, objectPath)
	if err != nil {
		return SignedUpload{}, err
	}
	if err := c.policy.Validate(opts.ContentType, opts.SizeBytes); err != nil {
		return SignedUpload{}, err
	}
	if !matchesContentType(objectPath, opts.ContentType) {
		return SignedUpload{}, ErrUnsupportedType
	}
	signed, err := s3.NewPresignClient(c.s3).PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(c.bucket), Key: aws.String(key),
		ContentType: aws.String(opts.ContentType), ContentLength: aws.Int64(opts.SizeBytes),
		CacheControl: aws.String("private, no-store"),
	}, func(o *s3.PresignOptions) { o.Expires = uploadLifetime })
	if err != nil {
		return SignedUpload{}, ErrSignFailed
	}
	headers := make(map[string]string)
	for name, values := range signed.SignedHeader {
		if !strings.EqualFold(name, "Host") {
			headers[name] = strings.Join(values, ",")
		}
	}
	return SignedUpload{URL: signed.URL, Path: strings.Trim(objectPath, "/"), Method: http.MethodPut, Headers: headers, ExpiresAt: time.Now().UTC().Add(uploadLifetime)}, nil
}

func (c *Client) DownloadURL(ctx context.Context, namespace, objectPath string) (string, error) {
	key, err := c.objectKey(namespace, objectPath)
	if err != nil {
		return "", err
	}
	signed, err := s3.NewPresignClient(c.s3).PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket), Key: aws.String(key),
	}, func(o *s3.PresignOptions) { o.Expires = downloadLifetime })
	if err != nil {
		return "", ErrSignFailed
	}
	return signed.URL, nil
}

func (c *Client) HeadObject(ctx context.Context, namespace, objectPath string) (ObjectInfo, error) {
	key, err := c.objectKey(namespace, objectPath)
	if err != nil {
		return ObjectInfo{}, err
	}
	out, err := c.s3.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(c.bucket), Key: aws.String(key)})
	if err != nil {
		var status interface{ HTTPStatusCode() int }
		var apiErr smithy.APIError
		if (errors.As(err, &status) && status.HTTPStatusCode() == http.StatusNotFound) ||
			(errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NotFound" || apiErr.ErrorCode() == "NoSuchKey")) {
			return ObjectInfo{}, ErrNotUploaded
		}
		return ObjectInfo{}, ErrInspectFailed
	}
	return ObjectInfo{ContentType: aws.ToString(out.ContentType), SizeBytes: aws.ToInt64(out.ContentLength)}, nil
}

func (c *Client) Delete(ctx context.Context, namespace string, objectPaths []string) error {
	if len(objectPaths) == 0 {
		return ErrInvalidObjectPath
	}
	// Validate the whole request before mutating storage.
	keys := make([]string, len(objectPaths))
	for i, path := range objectPaths {
		key, err := c.objectKey(namespace, path)
		if err != nil {
			return err
		}
		keys[i] = key
	}
	for _, key := range keys {
		_, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(c.bucket), Key: aws.String(key)})
		if err != nil {
			return ErrDeleteFailed
		}
	}
	return nil
}

func (c *Client) objectKey(namespace, objectPath string) (string, error) {
	if c == nil {
		return "", ErrNotConfigured
	}
	if c.configErr != nil {
		return "", c.configErr
	}
	if c.s3 == nil || c.bucket == "" {
		return "", ErrNotConfigured
	}
	if strings.Trim(objectPath, "/") == "" {
		return "", ErrInvalidObjectPath
	}
	if namespace == "" || strings.ContainsAny(namespace, "/\\") || namespace == "." || namespace == ".." {
		return "", ErrInvalidObjectPath
	}
	logical, _, err := cleanObjectPath(namespace + "/" + strings.Trim(objectPath, "/"))
	if err != nil {
		return "", err
	}
	return logical, nil
}

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
		for _, r := range part {
			if r < 32 || r == 127 {
				return "", "", ErrInvalidObjectPath
			}
		}
		escaped[i] = url.PathEscape(part)
	}
	return objectPath, strings.Join(escaped, "/"), nil
}

func matchesContentType(path, contentType string) bool {
	ext, ok := MediaExt(contentType)
	return ok && strings.HasSuffix(path, ext)
}
