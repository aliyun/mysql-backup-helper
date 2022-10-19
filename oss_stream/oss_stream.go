package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

var waitSender sync.WaitGroup
var imur oss.InitiateMultipartUploadResult
var size int

type OssProgressListener struct {
}

func (listener *OssProgressListener) ProgressChanged(event *oss.ProgressEvent) {
	switch event.EventType {
	case oss.TransferStartedEvent:
		fmt.Printf("Transfer Started, ConsumedBytes: %d, TotalBytes %d.\n",
			event.ConsumedBytes, event.TotalBytes)
	case oss.TransferDataEvent:
		fmt.Printf("\rTransfer Data, ConsumedBytes: %d, TotalBytes %d, %d%%.",
			event.ConsumedBytes, event.TotalBytes, event.ConsumedBytes*100/event.TotalBytes)
	case oss.TransferCompletedEvent:
		fmt.Printf("\nTransfer Completed, ConsumedBytes: %d, TotalBytes %d.\n",
			event.ConsumedBytes, event.TotalBytes)
	case oss.TransferFailedEvent:
		fmt.Printf("\nTransfer Failed, ConsumedBytes: %d, TotalBytes %d.\n",
			event.ConsumedBytes, event.TotalBytes)
	default:
	}
}

func producer(c chan []byte) {

	reader := bufio.NewReaderSize(os.Stdin, size)
	var data []byte
	index := 1

	for {
		p := make([]byte, size)
		n, err := io.ReadFull(reader, p)

		if n > 0 {
			data = p[:n]
		}

		dataLength := len(data)
		fmt.Println("[oss_stream] index", index, "bytes:", dataLength)

		if err != nil || dataLength == 0 {
			//last one
			if dataLength > 0 {
				waitSender.Add(1)
				c <- data
			}

			fmt.Println("[oss_stream] End With:", err)
			close(c)
			break
		}

		waitSender.Add(1)
		c <- data
		index += 1
	}
}

func sender(data []byte, index int, bucket *oss.Bucket, traffic int64) (part oss.UploadPart) {

	reader := bytes.NewReader(data)
	part, err := bucket.UploadPart(imur, reader, int64(len(data)), index, oss.Progress(&OssProgressListener{}), oss.TrafficLimitHeader(traffic))
	if err != nil {
		fmt.Println("Error:", err)
		err = bucket.AbortMultipartUpload(imur)
		if err != nil {
			fmt.Println("Error:", err)
		}
		os.Exit(-1)
	}
	return part
}

func consumer(ch chan []byte, bucket *oss.Bucket, parts_ch chan []oss.UploadPart, traffic int64) {

	var parts []oss.UploadPart
	index := 1

	for {
		if data, ok := <-ch; ok {
			part := sender(data, index, bucket, traffic)
			parts = append(parts, part)
			index += 1
			waitSender.Done()
		} else {
			break
		}
	}

	//等待上传任务完成
	fmt.Println("[oss_stream] wait sended ...")
	waitSender.Wait()
	parts_ch <- parts
}

func main() {
	var endpoint string
	var accessKeyId string
	var accessKeySecret string
	var securityToken string
	var bucketName string
	var objectName string
	var buffer int
	var traffic int64

	flag.StringVar(&endpoint, "endpoint", "http://oss-cn-hangzhou.aliyuncs.com", "oss endpoint")
	flag.StringVar(&accessKeyId, "accessKeyId", "", "accessKeyId")
	flag.StringVar(&accessKeySecret, "accessKeySecret", "", "accessKeySecret")
	flag.StringVar(&securityToken, "securityToken", "", "securityToken")
	flag.StringVar(&bucketName, "bucketName", "", "bucketName")
	flag.StringVar(&objectName, "objectName", "", "objectName")
	flag.IntVar(&size, "size", 1024*1024*100, "upload:block size default 100Mb")
	flag.IntVar(&buffer, "buffer", 10, "buffer * size = used memory")
	flag.Int64Var(&traffic, "traffic", 83886080, "limit upload traffic:default 10MB")

	flag.Parse()

	if endpoint == "" {
		panic("endpoint required")
	}
	if objectName == "" {
		panic("objectName required")
	}
	if accessKeyId == "" {
		panic("accessKeyId required")
	}
	if accessKeySecret == "" {
		panic("accessKeySecret required")
	}
	if bucketName == "" {
		panic("bucketName required")
	}

	waitSender = sync.WaitGroup{}
	var client *oss.Client
	var err error
	if securityToken == "" {
		client, err = oss.New(endpoint, accessKeyId, accessKeySecret)
	} else {
		client, err = oss.New(endpoint, accessKeyId, accessKeySecret, oss.SecurityToken(securityToken))
	}
	if err != nil {
		panic(err)
	}
	// 获取存储空间。
	bucket, err := client.Bucket(bucketName)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(-1)
	}

	storageType := oss.ObjectStorageClass(oss.StorageStandard)
	imur, _ = bucket.InitiateMultipartUpload(objectName, storageType)

	queue := make(chan []byte, buffer)
	partsCh := make(chan []oss.UploadPart, 1)

	/** run streaming  */
	go producer(queue)
	go consumer(queue, bucket, partsCh, traffic)

	/** watch kill signal */
	ch := make(chan os.Signal)
	go func() {
		signal.Notify(ch, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGUSR1, syscall.SIGUSR2)
		if _, ok := <-ch; ok {
			fmt.Println("service ending")
		}
		err = bucket.AbortMultipartUpload(imur)
		os.Exit(-1)
	}()

	/** complete streaming  */
	objectAcl := oss.ObjectACL(oss.ACLPrivate)
	parts := <-partsCh
	fmt.Println("parts", len(parts))

	cmur, err := bucket.CompleteMultipartUpload(imur, parts, objectAcl)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(-1)
	}

	fmt.Println("cmur:", cmur)
	fmt.Println("done")
}
