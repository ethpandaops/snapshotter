package s3

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/ethpandaops/eth-snapshotter/internal/config"
	log "github.com/sirupsen/logrus"
)

// S3Client is a client for interacting with S3 storage
type S3Client struct {
	client      *s3.Client
	endpoint    string
	region      string
	bucketName  string
	accessKey   string
	secretKey   string
	initialized bool
	cfg         *config.S3Config
}

// NewS3Client creates a new S3 client
func NewS3Client(cfg *config.S3Config) *S3Client {
	return &S3Client{
		cfg: cfg,
	}
}

// Initialize initializes the S3 client
func (c *S3Client) Initialize() error {
	// Use configuration if available
	if c.cfg != nil {
		if c.cfg.Endpoint != "" {
			c.endpoint = c.cfg.Endpoint
		}
		if c.cfg.Region != "" {
			c.region = c.cfg.Region
		}
		if c.cfg.BucketName != "" {
			c.bucketName = c.cfg.BucketName
		}
	}

	// Fall back to environment variables if not set in config
	if c.endpoint == "" {
		// Try S3_ENDPOINT_URL first
		c.endpoint = os.Getenv("S3_ENDPOINT_URL")

		if c.endpoint == "" {
			return fmt.Errorf("S3 endpoint not set in config or environment variables")
		}
	}

	if c.region == "" {
		// Try AWS_DEFAULT_REGION first
		c.region = os.Getenv("AWS_DEFAULT_REGION")

		if c.region == "" {
			c.region = "us-east-1" // Default region
		}
	}

	if c.bucketName == "" {
		// Try S3_BUCKET_NAME first
		c.bucketName = os.Getenv("S3_BUCKET_NAME")
	}

	// Get access credentials from environment
	c.accessKey = os.Getenv("AWS_ACCESS_KEY_ID")
	if c.accessKey == "" {
		return fmt.Errorf("access key not set in AWS_ACCESS_KEY_ID environment variable")
	}

	c.secretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	if c.secretKey == "" {

		return fmt.Errorf("secret key not set in AWS_SECRET_ACCESS_KEY environment variable")

	}

	// Create the AWS config
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(c.accessKey, c.secretKey, "")),
		awsconfig.WithRegion(c.region),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create the S3 client with service options for custom endpoint
	c.client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(c.endpoint)
		o.UsePathStyle = true
	})
	c.initialized = true

	log.WithFields(log.Fields{
		"endpoint": c.endpoint,
		"region":   c.region,
		"bucket":   c.bucketName,
	}).Info("initialized S3 client")

	return nil
}

// ensureInitialized ensures the client is initialized before use
func (c *S3Client) ensureInitialized() error {
	if !c.initialized {
		return c.Initialize()
	}
	return nil
}

// GetBucketName returns the configured bucket name
func (c *S3Client) GetBucketName() string {
	return c.bucketName
}

// GetEndpoint returns the configured endpoint
func (c *S3Client) GetEndpoint() string {
	return c.endpoint
}

// GetRegion returns the configured region
func (c *S3Client) GetRegion() string {
	return c.region
}

// GetRootPrefix returns the configured root prefix with a trailing slash if not empty
func (c *S3Client) GetRootPrefix() string {
	if c.cfg != nil && c.cfg.RootPrefix != "" {
		prefix := c.cfg.RootPrefix
		if !strings.HasSuffix(prefix, "/") {
			prefix = prefix + "/"
		}
		return prefix
	}
	return ""
}

// ParseS3URI parses an S3 URI into bucket and key
// Format: s3://bucket/key
func ParseS3URI(uri string) (bucket, key string, err error) {
	if !strings.HasPrefix(uri, "s3://") {
		return "", "", fmt.Errorf("invalid S3 URI format: %s", uri)
	}

	path := strings.TrimPrefix(uri, "s3://")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid S3 URI format: %s", uri)
	}

	return parts[0], parts[1], nil
}

// DeleteObject deletes a single object from S3
func (c *S3Client) DeleteObject(ctx context.Context, bucket, key string) error {
	if err := c.ensureInitialized(); err != nil {
		return err
	}

	// Use default bucket if not specified
	if bucket == "" {
		if c.bucketName == "" {
			return fmt.Errorf("bucket name not specified and no default bucket configured")
		}
		bucket = c.bucketName
	}

	_, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete S3 object %s/%s: %w", bucket, key, err)
	}

	log.WithFields(log.Fields{
		"bucket": bucket,
		"key":    key,
	}).Debug("Deleted S3 object")

	return nil
}

// DeleteDirectory deletes all objects under a prefix (simulating a directory)
func (c *S3Client) DeleteDirectory(ctx context.Context, bucket, prefix string) error {
	if err := c.ensureInitialized(); err != nil {
		return err
	}

	// Use default bucket if not specified
	if bucket == "" {
		if c.bucketName == "" {
			return fmt.Errorf("bucket name not specified and no default bucket configured")
		}
		bucket = c.bucketName
	}

	// Ensure the prefix ends with a slash
	if !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}

	// Collect all objects to delete
	var objectsToDelete []types.ObjectIdentifier

	paginator := s3.NewListObjectsV2Paginator(c.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})

	// List all objects with the given prefix
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list S3 objects: %w", err)
		}

		for _, obj := range page.Contents {
			objectsToDelete = append(objectsToDelete, types.ObjectIdentifier{
				Key: obj.Key,
			})
		}
	}

	// If no objects found, we're done
	if len(objectsToDelete) == 0 {
		log.WithFields(log.Fields{
			"bucket": bucket,
			"prefix": prefix,
		}).Info("No objects found to delete")
		return nil
	}

	log.WithFields(log.Fields{
		"bucket": bucket,
		"prefix": prefix,
		"count":  len(objectsToDelete),
	}).Info("Deleting objects")

	// Delete objects in batches of 1000 (S3 API limit)
	for i := 0; i < len(objectsToDelete); i += 1000 {
		end := i + 1000
		if end > len(objectsToDelete) {
			end = len(objectsToDelete)
		}

		batch := objectsToDelete[i:end]
		_, err := c.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(bucket),
			Delete: &types.Delete{
				Objects: batch,
				Quiet:   aws.Bool(true),
			},
		})
		if err != nil {
			return fmt.Errorf("failed to delete batch of S3 objects: %w", err)
		}
	}

	log.WithFields(log.Fields{
		"bucket": bucket,
		"prefix": prefix,
		"count":  len(objectsToDelete),
	}).Info("Successfully deleted all objects")

	return nil
}

// PutObject uploads content to S3
func (c *S3Client) PutObject(ctx context.Context, bucket, key string, content []byte) error {
	if err := c.ensureInitialized(); err != nil {
		return err
	}

	// Use default bucket if not specified
	if bucket == "" {
		if c.bucketName == "" {
			return fmt.Errorf("bucket name not specified and no default bucket configured")
		}
		bucket = c.bucketName
	}

	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader(string(content)),
	})
	if err != nil {
		return fmt.Errorf("failed to upload S3 object %s/%s: %w", bucket, key, err)
	}

	log.WithFields(log.Fields{
		"bucket": bucket,
		"key":    key,
	}).Debug("Uploaded S3 object")

	return nil
}
