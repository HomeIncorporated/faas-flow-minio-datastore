package MinioDataStore

import (
	"bytes"
	"fmt"
	minio "github.com/minio/minio-go"
	faasflow "github.com/s8sg/faasflow"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

type MinioDataStore struct {
	region      string
	bucketName  string
	flowName    string
	requestId   string
	minioClient *minio.Client
}

// GetMinioDataStore Initialize a minio DataStore object based on configuration
// Depends on s3_url, s3-secret-key, s3-access-key, [s3_bucket, s3_region](optional), workflow_name
func GetMinioDataStore() (faasflow.DataStore, error) {

	minioDataStore := &MinioDataStore{}

	minioDataStore.region = regionName()

	minioClient, connectErr := connectToMinio(minioDataStore.region)
	if connectErr != nil {
		return nil, fmt.Errorf("Failed to initialize minio, error %s", connectErr.Error())
	}
	minioDataStore.minioClient = minioClient

	return minioDataStore, nil
}

func (minioStore *MinioDataStore) Init(flowName string, requestId string) error {
	if minioStore.minioClient == nil {
		return fmt.Errorf("minio client not initialized, use GetMinioDataStore()")
	}

	minioStore.flowName = flowName
	minioStore.requestId = requestId

	bucketName := fmt.Sprintf("faasflow-%s-%s", flowName, requestId)

	minioStore.bucketName = bucketName
	err := minioStore.minioClient.MakeBucket(bucketName, minioStore.region)
	if err != nil {
		return fmt.Errorf("error creating: %s, error: %s", minioStore.bucketName, err.Error())
	}

	return nil
}

func (minioStore *MinioDataStore) Set(key string, value string) error {
	if minioStore.minioClient == nil {
		return fmt.Errorf("minio client not initialized, use GetMinioDataStore()")
	}

	fullPath := getPath(minioStore.bucketName, key)
	reader := bytes.NewReader([]byte(value))
	_, err := minioStore.minioClient.PutObject(minioStore.bucketName,
		fullPath,
		reader,
		int64(reader.Len()),
		minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("error writing: %s, error: %s", fullPath, err.Error())
	}

	return nil
}

func (minioStore *MinioDataStore) Get(key string) (string, error) {
	if minioStore.minioClient == nil {
		return "", fmt.Errorf("minio client not initialized, use GetMinioDataStore()")
	}

	fullPath := getPath(minioStore.bucketName, key)
	obj, err := minioStore.minioClient.GetObject(minioStore.bucketName, fullPath, minio.GetObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("error reading: %s, error: %s", fullPath, err.Error())
	}

	data, _ := ioutil.ReadAll(obj)

	return string(data), nil
}

func (minioStore *MinioDataStore) Del(key string) error {
	if minioStore.minioClient == nil {
		return fmt.Errorf("minio client not initialized, use GetMinioDataStore()")
	}

	fullPath := getPath(minioStore.bucketName, key)
	err := minioStore.minioClient.RemoveObject(minioStore.bucketName, fullPath)
	if err != nil {
		return fmt.Errorf("error removing: %s, error: %s", fullPath, err.Error())
	}
	return nil
}

func (minioStore *MinioDataStore) Cleanup() error {
	err := minioStore.minioClient.RemoveBucket(minioStore.bucketName)
	if err != nil {
		return fmt.Errorf("error removing: %s, error: %s", minioStore.bucketName, err.Error())
	}
	return nil
}

func readSecret(key string) (string, error) {
	basePath := "/var/openfaas/secrets/"
	if len(os.Getenv("secret_mount_path")) > 0 {
		basePath = os.Getenv("secret_mount_path")
	}

	readPath := path.Join(basePath, key)
	secretBytes, readErr := ioutil.ReadFile(readPath)
	if readErr != nil {
		return "", fmt.Errorf("unable to read secret: %s, error: %s", readPath, readErr)
	}
	val := strings.TrimSpace(string(secretBytes))
	return val, nil
}

func connectToMinio(region string) (*minio.Client, error) {

	endpoint := os.Getenv("s3_url")

	tlsEnabled := tlsEnabled()

	secretKey, err := readSecret("s3-secret-key")
	accessKey, err := readSecret("s3-access-key")
	if err != nil {
		return nil, err
	}

	return minio.New(endpoint, accessKey, secretKey, tlsEnabled)
}

// getPath produces a string as <bucketname>/<flowname>/<requestId>/<key>.value
func getPath(bucket, key string) string {
	fileName := fmt.Sprintf("%s.value", key)
	return fmt.Sprintf("%s/key/%s", bucket, fileName)
}

func tlsEnabled() bool {
	if connection := os.Getenv("s3_tls"); connection == "true" || connection == "1" {
		return true
	}
	return false
}

func regionName() string {
	regionName, exist := os.LookupEnv("s3_region")
	if exist == false || len(regionName) == 0 {
		regionName = "us-east-1"
	}
	return regionName
}
