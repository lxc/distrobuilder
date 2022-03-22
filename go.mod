module github.com/lxc/distrobuilder

go 1.16

exclude github.com/rootless-containers/proto v0.1.0

replace google.golang.org/grpc/naming => google.golang.org/grpc v1.29.1

require (
	github.com/flosch/pongo2 v0.0.0-20200913210552-0d938eb266f3
	github.com/gobuffalo/packr/v2 v2.8.3
	github.com/lxc/lxd v0.0.0-20220321221753-1e535dcd5db5
	github.com/mudler/docker-companion v0.4.6-0.20211015133729-bd4704fad372
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.4.0
	github.com/stretchr/testify v1.7.1
	golang.org/x/sys v0.0.0-20220319134239-a9b59b0215f8
	gopkg.in/antchfx/htmlquery.v1 v1.2.2
	gopkg.in/flosch/pongo2.v3 v3.0.0-20141028000813-5e81b817a0c4
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/AdaLogics/go-fuzz-headers v0.0.0-20220301192128-fe11a1f79e80 // indirect
	github.com/antchfx/xpath v1.2.0 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.11.3 // indirect
	github.com/docker/distribution v2.8.1+incompatible // indirect
	github.com/docker/docker v20.10.13+incompatible // indirect
	github.com/fsouza/go-dockerclient v1.7.10 // indirect
	github.com/google/go-containerregistry v0.8.0 // indirect
	github.com/heroku/docker-registry-client v0.0.0-20211012143308-9463674c8930 // indirect
	github.com/moby/sys/mount v0.3.1 // indirect
	github.com/opencontainers/umoci v0.4.8-0.20211009121349-9c76304c034d // indirect
	github.com/urfave/cli v1.22.5 // indirect
	golang.org/x/crypto v0.0.0-20220321153916-2c7772ba3064 // indirect
	google.golang.org/protobuf v1.28.0 // indirect
)
