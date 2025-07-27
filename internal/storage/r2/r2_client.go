package r2

import (
    "bytes"
    "fmt"
    "io"
    "net/url"

    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/credentials"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/s3"
)

// Client обёртка над AWS S3-клиентом для Cloudflare R2
// Положить в каталог internal/storage/r2/r2_client.go

type Client struct {
    svc    *s3.S3
    bucket string
    host   string
}

// NewClient создаёт новый R2-клиент
func NewClient(endpoint, accessKey, secretKey, bucket string) (*Client, error) {
    cfg := aws.NewConfig().
        WithRegion("auto").
        WithEndpoint(endpoint).
        WithCredentials(credentials.NewStaticCredentials(accessKey, secretKey, "")).
        WithS3ForcePathStyle(true)

    sess, err := session.NewSession(cfg)
    if err != nil {
        return nil, fmt.Errorf("failed to create R2 session: %w", err)
    }

    // Парсим endpoint, чтобы получить host для URL
    u, err := url.Parse(endpoint)
    if err != nil {
        return nil, fmt.Errorf("invalid R2 endpoint URL: %w", err)
    }

    return &Client{
        svc:    s3.New(sess),
        bucket: bucket,
        host:   u.Host,
    }, nil
}

// Upload загружает данные в R2 под ключом objectKey и возвращает публичный URL
func (c *Client) Upload(objectKey string, data []byte) (string, error) {
    _, err := c.svc.PutObject(&s3.PutObjectInput{
        Bucket: aws.String(c.bucket),
        Key:    aws.String(objectKey),
        Body:   bytes.NewReader(data),
        ACL:    aws.String("public-read"),
    })
    if err != nil {
        return "", fmt.Errorf("failed to upload to R2: %w", err)
    }

    // Формируем публичный URL: https://<host>/<bucket>/<objectKey>
    return fmt.Sprintf("https://%s/%s/%s", c.host, c.bucket, objectKey), nil
}

// Download скачивает объект из R2 по ключу objectKey
func (c *Client) Download(objectKey string) ([]byte, error) {
    out, err := c.svc.GetObject(&s3.GetObjectInput{
        Bucket: aws.String(c.bucket),
        Key:    aws.String(objectKey),
    })
    if err != nil {
        return nil, fmt.Errorf("failed to download from R2: %w", err)
    }
    defer out.Body.Close()

    buf := new(bytes.Buffer)
    if _, err := io.Copy(buf, out.Body); err != nil {
        return nil, fmt.Errorf("failed reading R2 response body: %w", err)
    }
    return buf.Bytes(), nil
}