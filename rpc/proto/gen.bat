REM API Source Code Generator for SKT Client API Developers
REM Run this script ONLY on Windows
REM See README.md for details
set ver=v1.16.0
protoc -I=%~dp0 ^
    -I %GOPATH%/pkg/mod/github.com/grpc-ecosystem/grpc-gateway@%ver%/third_party/googleapis ^
    -I %GOPATH%/pkg/mod/github.com/grpc-ecosystem/grpc-gateway@%ver% ^
    --gogo_out=plugins=grpc:. ^
    %~dp0\api.proto

protoc -I=%~dp0 ^
    -I %GOPATH%/pkg/mod/github.com\grpc-ecosystem\grpc-gateway@%ver%\third_party\googleapis ^
    -I %GOPATH%/pkg/mod/github.com\grpc-ecosystem\grpc-gateway@%ver% ^
    --grpc-gateway_out=logtostderr=true:. ^
    %~dp0\api.proto

protoc -I=%~dp0 ^
    -I %GOPATH%/pkg/mod/github.com\grpc-ecosystem\grpc-gateway@%ver%\third_party\googleapis ^
    -I %GOPATH%/pkg/mod/github.com\grpc-ecosystem\grpc-gateway@%ver% ^
    --swagger_out=logtostderr=true:. ^
    %~dp0\api.proto