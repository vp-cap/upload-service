module cap/upload-service

go 1.14

require (
	cap/data-lib v0.0.0-00010101000000-000000000000
	github.com/golang/protobuf v1.4.2
	github.com/spf13/viper v1.3.2
	google.golang.org/grpc v1.30.0
	google.golang.org/protobuf v1.23.0
)

replace cap/data-lib => ../data-lib

replace cap/upload-service => ./
