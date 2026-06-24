package artifacts

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/sirupsen/logrus"

	"picotera/pkg/configx"
)

type Sink interface {
	Put(ctx context.Context, key string, payload []byte)
	PresignedGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	Enabled() bool
}

// bucketLookup maps the tri-state PICOTERA_S3_PATH_STYLE setting to a minio
// BucketLookupType: nil = auto-detect, true = force path style, false = force
// virtual-hosted (DNS) style.
func bucketLookup(pathStyle *bool) minio.BucketLookupType {
	if pathStyle == nil {
		return minio.BucketLookupAuto
	}
	if *pathStyle {
		return minio.BucketLookupPath
	}
	return minio.BucketLookupDNS
}

func NewSink(cfg configx.S3Config, logger *logrus.Entry) (Sink, error) {
	if cfg.Endpoint == "" {
		logger.Info("artifact disabled (PICOTERA_S3_ENDPOINT empty)")
		return noopSink{}, nil
	}
	if cfg.AccessKey == "" || cfg.SecretKey == "" || cfg.Bucket == "" {
		return nil, fmt.Errorf("artifact: s3 access_key, secret_key, bucket must be set when endpoint is configured")
	}
	lookup := bucketLookup(cfg.PathStyle)
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:       cfg.UseSSL,
		Region:       cfg.Region,
		BucketLookup: lookup,
	})
	if err != nil {
		return nil, fmt.Errorf("artifact: create minio client: %w", err)
	}
	u, err := url.Parse(cfg.PublicURL)
	if err != nil {
		return nil, fmt.Errorf("artifact: parse public url: %w", err)
	}
	urlSignerClient, err := minio.New(u.Host, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:       u.Scheme == "https",
		Region:       cfg.Region,
		BucketLookup: lookup,
	})
	if err != nil {
		return nil, fmt.Errorf("artifact: create minio url signer client: %w", err)
	}
	s := &minioSink{
		client:          client,
		urlSignerClient: urlSignerClient,
		bucket:          cfg.Bucket,
		publicURL:       cfg.PublicURL,
		logger:          logger,
		jobs:            make(chan job, 256),
	}
	for i := 0; i < 4; i++ {
		go s.worker()
	}
	logger.WithField("bucket", cfg.Bucket).WithField("endpoint", cfg.Endpoint).Info("artifact sink ready")
	return s, nil
}

type noopSink struct{}

func (noopSink) Put(ctx context.Context, key string, payload []byte) {}
func (noopSink) PresignedGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return "", nil
}
func (noopSink) Enabled() bool { return false }

type job struct {
	key     string
	payload []byte
}

type minioSink struct {
	client          *minio.Client
	urlSignerClient *minio.Client
	bucket          string
	publicURL       string
	logger          *logrus.Entry
	jobs            chan job
}

func (s *minioSink) Enabled() bool { return true }

func (s *minioSink) Put(ctx context.Context, key string, payload []byte) {
	select {
	case s.jobs <- job{key: key, payload: payload}:
	default:
		s.logger.WithField("key", key).Warn("artifact: queue full, dropping")
	}
}

func (s *minioSink) worker() {
	for j := range s.jobs {
		s.upload(j)
	}
}

func (s *minioSink) upload(j job) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := s.client.PutObject(ctx, s.bucket, j.key, bytes.NewReader(j.payload), int64(len(j.payload)), minio.PutObjectOptions{
		ContentType:     "application/json",
		ContentEncoding: "zstd",
	})
	if err != nil {
		s.logger.WithError(err).WithField("key", j.key).Warn("artifact: upload failed")
	}
}

func (s *minioSink) PresignedGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	u, err := s.urlSignerClient.PresignedGetObject(ctx, s.bucket, key, ttl, nil)
	if err != nil {
		return "", err
	}
	if s.publicURL == "" {
		return u.String(), nil
	}
	pub, err := url.Parse(s.publicURL)
	if err != nil {
		return "", fmt.Errorf("artifact: parse public url: %w", err)
	}
	u.Scheme = pub.Scheme
	u.Host = pub.Host
	if pub.Path != "" && pub.Path != "/" {
		u.Path = pub.Path + u.Path
	}
	return u.String(), nil
}
