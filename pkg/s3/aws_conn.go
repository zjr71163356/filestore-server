package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func main() {
	ctx := context.TODO()

	// 1. 加载默认配置（它会自动寻找环境变量里的 ACCESS_KEY 和 SECRET_KEY）
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}

	// 2. 创建 S3 客户端
	s3Client := s3.NewFromConfig(cfg)

	// 3. 打开本地文件
	file, err := os.Open("cat_photo.jpg")
	if err != nil {
		log.Fatalf("打开文件失败: %v", err)
	}
	defer file.Close()

	// 4. 上传对象
	bucketName := "my-global-cat-bucket"
	objectKey := "images/cute_kitten.jpg"

	_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
		Body:   file,
	})

	if err != nil {
		log.Fatalf("上传失败，喵呜: %v", err)
	}

	fmt.Printf("成功将照片上传到 S3 的 %s/%s！\n", bucketName, objectKey)
}
