@echo off
set CGO_ENABLED=1
set PATH=C:\Users\PC\AppData\Local\Microsoft\WinGet\Packages\BrechtSanders.WinLibs.POSIX.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe\mingw64\bin;C:\msys64\mingw64\bin;%PATH%
echo [zUtility] Building...

:: Copy icon
echo [icon] Copying assets\icon.png -> pkg\embeddable\icon.png
copy /Y assets\icon.png pkg\embeddable\icon.png >nul

:: Try to generate app.syso (requires rsrc tool), skip if not available
where rsrc >nul 2>nul
if %errorlevel% == 0 (
    echo [icon] Generating app.syso from assets\icon.ico
    rsrc -manifest app.exe.manifest -ico assets\icon.ico -o app.syso
) else (
    echo [icon] rsrc not found - skipping icon embedding
)

:: Build
go build -ldflags="-s -w -H=windowsgui" -tags no_emoji -o zUtility.exe .
if %errorlevel% neq 0 (
    echo [ERROR] Build failed!
    pause
    exit /b 1
)

echo [zUtility] Done! Output: zUtility.exe
pause
