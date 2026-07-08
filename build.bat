@echo off
set PATH=C:\msys64\mingw64\bin;C:\Users\famac\go\bin;%PATH%
for /f "tokens=*" %%i in ('git describe --tags --always') do set GIT_VERSION=%%i
for /f "tokens=*" %%i in ('git rev-parse --short HEAD') do set GIT_COMMIT=%%i

set PKG=github.com/something-that-is-cool/zutil/internal/version

if not exist app.syso (
    go-winres simply --arch amd64 --icon app/icon.png --out app.syso --no-suffix --admin
)
go build -ldflags="-s -w -H=windowsgui -X '%PKG%.Version=%GIT_VERSION%' -X '%PKG%.Commit=%GIT_COMMIT%'" -tags no_emoji -o zutil.exe