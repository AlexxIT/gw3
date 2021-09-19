@echo off
set GOOS=linux
set GOARCH=mipsle
go build -ldflags "-s -w" -trimpath -o gw3 ..
upx -q gw3

if "%1"=="" exit /b

rem Upload binary to gateway if pass gate IP-address as first param
rem /data/busybox tcpsvd -E 0.0.0.0 21 /data/busybox ftpd -w &
ftp -s:upload.txt %1
