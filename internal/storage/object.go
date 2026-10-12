package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const bucket = "files"

// Put кладёт data объектом записи id владельца owner.
func (c *Client) Put(ctx context.Context, owner, id string, data []byte) error {
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(owner + "/" + id),
		Body: bytes.NewReader(data)})
	return err
}

// Get читает объект записи id владельца owner не длиннее limit байт.
// Отсутствие объекта отмечает ErrNoObject.
func (c *Client) Get(ctx context.Context, owner, id string, limit int64) ([]byte, error) {
	out, err := c.s3.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(owner + "/" + id)})
	if re, ok := errors.AsType[*awshttp.ResponseError](err); ok && re.HTTPStatusCode() == http.StatusNotFound {
		return nil, ErrNoObject
	}
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	data, err := io.ReadAll(io.LimitReader(out.Body, limit+1))
	if err == nil && int64(len(data)) > limit {
		err = errors.New("объект файла больше предела размера")
	}
	return data, err
}

// Delete удаляет объект записи id владельца owner. Отсутствие объекта не
// считается ошибкой.
func (c *Client) Delete(ctx context.Context, owner, id string) error {
	_, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(owner + "/" + id)})
	return err
}
