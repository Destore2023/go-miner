# development tool

## Remote debug

1. install delve 

```bash
go get -u github.com/go-delve/delve/cmd/dlv
# keep dlv in path
export PATH=$PATH:~/go/bin
```

2. build with debug mode

```bash
go build -gcflags "all=-N -l" ./
```

3. find miner pid

```bash
ps -aux | grep miner | grep -v 'grep' | awk '{print $2}'
```

or 

```bash
systemctl status skt-minerd
```

4. start debug server 

```bash
dlv debug --headless --listen=:2345 --api-version=2 --accept-multiclient
```
or 
```bash
dlv attach PID --headless --api-version=2 --log --listen=:2345
```

