@echo off
set GOOS=linux
set GOARCH=mipsle
go build -ldflags "-s -w" -trimpath -o bin\gw3
upx -q bin\gw3
ftp -s:bin\upload.txt
