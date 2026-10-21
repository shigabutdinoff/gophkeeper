package storage

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ErrNoObject сообщает, что объекта файла нет на сервере.
var ErrNoObject = errors.New("объекта файла нет на сервере")

// Client работает с объектами файлов на сервере.
type Client struct {
	s3 *s3.Client
}

// New собирает клиент хранилища файлов сервера server в регионе region.
// Запросы идут через client и подписываются ключом приложения key и
// токеном token.
func New(client *http.Client, server, region, key, token string) *Client {
	server = strings.TrimSuffix(server, "/")
	project, _, _ := strings.Cut(strings.TrimPrefix(server, "https://"), ".")
	return &Client{s3.New(s3.Options{
		BaseEndpoint: aws.String(server + "/storage/v1/s3"), UsePathStyle: true, Region: region,
		HTTPClient: client, Retryer: aws.NopRetryer{},
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: project, SecretAccessKey: key, SessionToken: token}, nil
		}),
	})}
}
