module github.com/tsuru/bs

go 1.15

require (
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78
	github.com/Graylog2/go-gelf v0.0.0-20170811154226-7ebf4f536d8f
	github.com/Microsoft/go-winio v0.4.11
	github.com/StackExchange/wmi v0.0.0-20150520194626-f3e2bae1e0cb
	github.com/ajg/form v0.0.0-20160429221517-e7b99d6ea9c5
	github.com/containerd/continuity v0.0.0-20180921161001-7f53d412b9eb
	github.com/docker/docker v17.12.0-ce-rc1.0.20180924202107-a9c061deec0f+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-units v0.3.3
	github.com/docker/libnetwork v0.8.0-dev.2.0.20180608203834-19279f049241
	github.com/fsouza/go-dockerclient v1.3.0
	github.com/garyburd/redigo v0.0.0-20140714215019-6628c86d6a89
	github.com/go-ole/go-ole v1.2.1-0.20160311030626-572eabb84c42
	github.com/gogo/protobuf v1.1.1
	github.com/google/gops v0.3.2-0.20170319002943-62f833fc9f6c
	github.com/gorilla/context v1.1.1
	github.com/gorilla/mux v1.6.2
	github.com/hashicorp/golang-lru v0.0.0-20150512181540-995efda3e073
	github.com/howeyc/fsnotify v0.9.0
	github.com/kardianos/osext v0.0.0-20150410034420-8fef92e41e22
	github.com/konsorten/go-windows-terminal-sequences v0.0.0-20180402223658-b729f2633dfe
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opencontainers/image-spec v1.0.1
	github.com/opencontainers/runc v0.1.2-0.20160519150036-419d5be191c1
	github.com/pkg/errors v0.8.0
	github.com/shirou/gopsutil v0.0.0-20160322125516-be06a94d4487
	github.com/shirou/w32 v0.0.0-20160930032740-bb4de0191aa4
	github.com/sirupsen/logrus v1.1.0
	github.com/tsuru/commandmocker v0.0.0-20150717135858-be4aec17ebc7
	github.com/tsuru/config v0.0.0-20151207200950-a4028d4efbb9
	github.com/tsuru/monsterqueue v0.0.0-20150730191847-c2240c7d35e4
	github.com/tsuru/tsuru v0.0.0-20160106221702-f0bfa8e74731
	golang.org/x/crypto v0.0.0-20180910181607-0e37d006457b
	golang.org/x/net v0.0.0-20180826012351-8a410e7b638d
	golang.org/x/sys v0.0.0-20180925112736-b09afc3d579e
	gopkg.in/check.v1 v1.0.0-20141024133853-64131543e789
	gopkg.in/mcuadros/go-syslog.v2 v2.2.1
	gopkg.in/mgo.v2 v2.0.0-20151207021513-e30de8ac9ae3
	gopkg.in/yaml.v1 v1.0.0-20140924161607-9f9df34309c0
)

replace (
	github.com/Graylog2/go-gelf v0.0.0-20170811154226-7ebf4f536d8f => github.com/cezarsa/go-gelf v0.0.0-20181026022425-1ff80b6cbc53
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 => github.com/ijc25/Gotty v0.0.0-20170406111628-a8b993ba6abd
)
