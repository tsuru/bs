module github.com/tsuru/bs

go 1.15

require (
	github.com/Graylog2/go-gelf v0.0.0-20170811154226-7ebf4f536d8f
	github.com/StackExchange/wmi v0.0.0-20150520194626-f3e2bae1e0cb // indirect
	github.com/ajg/form v0.0.0-20160429221517-e7b99d6ea9c5
	github.com/bradfitz/go-smtpd v0.0.0-20170404230938-deb6d6237625 // indirect
	github.com/containerd/continuity v0.0.0-20180921161001-7f53d412b9eb // indirect
	github.com/docker/docker v17.12.0-ce-rc1.0.20180924202107-a9c061deec0f+incompatible // indirect
	github.com/fsouza/go-dockerclient v1.3.0
	github.com/garyburd/redigo v0.0.0-20140714215019-6628c86d6a89 // indirect
	github.com/go-ole/go-ole v1.2.1-0.20160311030626-572eabb84c42 // indirect
	github.com/google/gops v0.3.2-0.20170319002943-62f833fc9f6c
	github.com/hashicorp/golang-lru v0.0.0-20150512181540-995efda3e073
	github.com/howeyc/fsnotify v0.9.0 // indirect
	github.com/kardianos/osext v0.0.0-20150410034420-8fef92e41e22 // indirect
	github.com/opencontainers/image-spec v1.0.2 // indirect
	github.com/opencontainers/runc v0.1.2-0.20160519150036-419d5be191c1 // indirect
	github.com/shirou/gopsutil v0.0.0-20160322125516-be06a94d4487
	github.com/shirou/w32 v0.0.0-20160930032740-bb4de0191aa4 // indirect
	github.com/sirupsen/logrus v1.1.0 // indirect
	github.com/tsuru/commandmocker v0.0.0-20150717135858-be4aec17ebc7
	github.com/tsuru/config v0.0.0-20151207200950-a4028d4efbb9 // indirect
	github.com/tsuru/monsterqueue v0.0.0-20150730191847-c2240c7d35e4 // indirect
	github.com/tsuru/tsuru v0.0.0-20160106221702-f0bfa8e74731
	golang.org/x/crypto v0.0.0-20180910181607-0e37d006457b // indirect
	golang.org/x/net v0.0.0-20180826012351-8a410e7b638d
	golang.org/x/sys v0.0.0-20180925112736-b09afc3d579e // indirect
	gopkg.in/check.v1 v1.0.0-20141024133853-64131543e789
	gopkg.in/mcuadros/go-syslog.v2 v2.2.1
	gopkg.in/mgo.v2 v2.0.0-20151207021513-e30de8ac9ae3 // indirect
	gopkg.in/yaml.v1 v1.0.0-20140924161607-9f9df34309c0 // indirect
)

replace (
	github.com/Graylog2/go-gelf v0.0.0-20170811154226-7ebf4f536d8f => github.com/cezarsa/go-gelf v0.0.0-20181026022425-1ff80b6cbc53
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 => github.com/ijc25/Gotty v0.0.0-20170406111628-a8b993ba6abd
)
