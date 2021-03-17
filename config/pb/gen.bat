@echo off
:: windows gen pb file
del /F/A/Q  *.pb.go
set ver=v1.3.2
protoc -I=. -I=%GOPATH%\src -I=%GOPATH%\pkg\mod\github.com\gogo\protobuf@%ver%\protobuf  ^
--plugin=protoc-gen-go=%GOPATH%\bin\protoc-gen-go ^
--gogo_out=.\  config.proto
