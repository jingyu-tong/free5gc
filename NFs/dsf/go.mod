module github.com/free5gc/dsf

go 1.25.5

require (
	github.com/free5gc/dataapi v0.0.0
	github.com/google/uuid v1.6.0
	github.com/sirupsen/logrus v1.9.3
	github.com/urfave/cli/v2 v2.27.7
	github.com/free5gc/util v1.3.2-0.20260107090449-c09baaf75b11
	google.golang.org/grpc v1.79.3
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/free5gc/dataapi => ../dataapi
