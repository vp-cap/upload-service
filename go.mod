module cap/upload-service

go 1.14

require (
	cap/data-lib v0.0.0-00010101000000-000000000000
	github.com/spf13/viper v1.3.2
)

replace cap/data-lib => ../data-lib

replace cap/upload-service/config => ./config
