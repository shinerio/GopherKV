@echo off
set BINARY_DIR=bin
set SERVER_BIN=%BINARY_DIR%\kvd.exe
set CLIENT_BIN=%BINARY_DIR%\kvcli.exe
set SERVER_SRC=.\cmd\kvd
set CLIENT_SRC=.\cmd\kvcli

if "%1"=="build" goto build
if "%1"=="clean" goto clean
if "%1"=="test" goto test
if "%1"=="help" goto help

goto build

:build
echo Building...
if not exist "%BINARY_DIR%" mkdir %BINARY_DIR%
go build -o %SERVER_BIN% %SERVER_SRC%
go build -o %CLIENT_BIN% %CLIENT_SRC%
echo Build complete!
goto end

:clean
echo Cleaning...
if exist "%BINARY_DIR%" rmdir /s /q %BINARY_DIR%
go clean -cache
echo Clean complete!
goto end

:test
echo Running tests...
go test -v ./...
goto end

:help
echo Available commands:
echo   build.bat        - Build all binaries (default)
echo   build.bat build  - Build kvd and kvcli
echo   build.bat clean  - Remove binaries and clean cache
echo   build.bat test   - Run all tests
echo   build.bat help   - Show this help message
goto end

:end
