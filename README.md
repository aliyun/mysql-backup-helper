MySQL Backup Helper
-----------

# 使用方法

## 前置检查

```sh
# 编译
go build -a -o backup-helper main.go

# 使用方法
./backup-helper -host [实例地址] -user [用户]  -port [端口] --password [密码]

# 帮助文档
./backup-helper -h
Usage of ./backup-helper:
  -host string
    	Connect to host (default "127.0.0.1")
  -password string
    	Password to use when connecting to server. If password is not given it's asked from the tty.
  -port int
    	Port number to use for connection (default 3306)
  -user string
    	User for login (default "root")
```

## 流式备份

```sh
cd oss_stream

# 编译
go build  -a -o oss_stream oss_stream.go

# 使用方法
innobackupex --backup --host=<host> --port=<port> --user=<dbuser> --password=<password> --stream=xbstream --compress /home/mysql/backup | oss_stream   

# 帮助文档
Usage of ./oss_stream:
  -accessKeyId string
        accessKeyId
  -accessKeySecret string
        accessKeySecret
  -bucketName string
        bucketName
  -buffer int
        buffer * size = used memory (default 10)
  -endpoint string
        oss endpoint (default "http://oss-cn-hangzhou.aliyuncs.com")
  -objectName string
        objectName
  -size int
        upload:block size default 100M (default 104857600)
```
