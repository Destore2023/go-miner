REM API Source Code Generator for SKT Client API Developers
REM Run this script ONLY on Windows
REM See README.md for details
protoc -I=%~dp0 ^
    -I %GOPATH%/pkg/mod/github.com/grpc-ecosystem/grpc-gateway@v1.15.2/third_party/googleapis ^
    -I %GOPATH%/pkg/mod/github.com/grpc-ecosystem/grpc-gateway@v1.15.2^
    --go_out=plugins=grpc:. ^
    %~dp0\api.proto

protoc -I=%~dp0 ^
    -I %GOPATH%/pkg/mod/github.com\grpc-ecosystem\grpc-gateway@v1.15.2\third_party\googleapis ^
    -I %GOPATH%/pkg/mod/github.com\grpc-ecosystem\grpc-gateway@v1.15.2 ^
    --grpc-gateway_out=logtostderr=true:. ^
    %~dp0\api.proto

protoc -I=%~dp0 ^
    -I %GOPATH%/pkg/mod/github.com\grpc-ecosystem\grpc-gateway@v1.15.2\third_party\googleapis ^
    -I %GOPATH%/pkg/mod/github.com\grpc-ecosystem\grpc-gateway@v1.15.2 ^
    --swagger_out=logtostderr=true:. ^
    %~dp0\api.proto