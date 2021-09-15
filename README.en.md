MySQL Backup Helper
-----------

# Usage

## Checking before backup

```sh
# compile
go build -a -o backup-helper main.go

# Usage
./backup-helper -host [db connection string] -user [db user]  -port [db port] --password [db password]

# Help
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

## Streaming Backup

```sh
cd oss_stream

# Compile
go build  -a -o oss_stream oss_stream.go

# Usage
innobackupex --backup --host=<host> --port=<port> --user=<dbuser> --password=<password> --stream=xbstream --compress /home/mysql/backup | oss_stream   

# Help
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
