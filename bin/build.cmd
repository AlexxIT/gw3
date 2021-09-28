@echo off
set GOOS=linux
set GOARCH=mipsle

for /f "tokens=1-3 delims=." %%a in ("%date%") do set DATE=%%c.%%b.%%a
for /f %%a in ('git rev-parse --short HEAD') do set COMMIT=%%a

go build -ldflags "-w -s -X main.version=%DATE%_%COMMIT%" -trimpath -o gw3 ..
upx -q gw3

if "%1"=="" exit /b

rem Upload binary to gateway if pass gate IP-address as first param
rem /data/busybox tcpsvd -E 0.0.0.0 21 /data/busybox ftpd -w &
ftp -s:upload.txt %1
