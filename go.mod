module github.com/juju/juju

go 1.13

require (
	github.com/Azure/azure-sdk-for-go v40.0.0+incompatible
	github.com/Azure/go-autorest/autorest v0.10.0
	github.com/Azure/go-autorest/autorest/adal v0.8.2
	github.com/Azure/go-autorest/autorest/date v0.2.0
	github.com/Azure/go-autorest/autorest/mocks v0.3.0
	github.com/Azure/go-autorest/autorest/to v0.3.0
	github.com/Azure/go-autorest/autorest/validation v0.2.0 // indirect
	github.com/EvilSuperstars/go-cidrman v0.0.0-20170211231153-4e5a4a63d9b7
	github.com/altoros/gosigma v0.0.0-00010101000000-000000000000
	github.com/armon/go-metrics v0.0.0-20180917152333-f0300d1749da
	github.com/aws/aws-sdk-go v1.29.8
	github.com/bmizerany/pat v0.0.0-20160217103242-c068ca2f0aac
	github.com/boltdb/bolt v1.3.1-0.20170131192018-e9cf4fae01b5 // indirect
	github.com/coreos/go-systemd/v22 v22.0.0
	github.com/dnaeon/go-vcr v1.0.1 // indirect
	github.com/docker/distribution v2.6.0-rc.1.0.20180522175653-f0cc92778478+incompatible
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/flosch/pongo2 v0.0.0-20141028000813-5e81b817a0c4 // indirect
	github.com/golang/mock v1.3.1
	github.com/google/go-cmp v0.4.0 // indirect
	github.com/google/go-querystring v0.0.0-20160401233042-9235644dd9e5
	github.com/googleapis/gnostic v0.2.0
	github.com/gorilla/handlers v0.0.0-20170224193955-13d73096a474
	github.com/gorilla/schema v0.0.0-20160426231512-08023a0215e7
	github.com/gorilla/websocket v1.4.0
	github.com/gosuri/uitable v0.0.1
	github.com/hashicorp/go-immutable-radix v1.0.0 // indirect
	github.com/hashicorp/go-msgpack v0.0.0-20150518234257-fa3f63826f7c
	github.com/hashicorp/raft v0.0.0-00010101000000-000000000000
	github.com/hashicorp/raft-boltdb v0.0.0-20171010151810-6e5ba93211ea
	github.com/imdario/mergo v0.3.6 // indirect
	github.com/joyent/gocommon v0.0.0-20160320193133-ade826b8b54e
	github.com/joyent/gosdc v0.0.0-20140524000815-2f11feadd2d9
	github.com/joyent/gosign v0.0.0-20140524000734-0da0d5f13420
	github.com/juju/ansiterm v0.0.0-20160907234532-b99631de12cf
	github.com/juju/bundlechanges v0.0.0-20191211033529-4ca814a87a32
	github.com/juju/clock v0.0.0-20190205081909-9c5c9712527c
	github.com/juju/cmd v0.0.0-20200108104440-8e43f3faa5c9
	github.com/juju/collections v0.0.0-20180717171555-9be91dc79b7c
	github.com/juju/description v0.0.0-20200127111509-d884fa94b910
	github.com/juju/errors v0.0.0-20190930114154-d42613fe1ab9
	github.com/juju/gnuflag v0.0.0-20171113085948-2ce1bb71843d
	github.com/juju/go-oracle-cloud v0.0.0-20170421134547-932a8cea00a1
	github.com/juju/go4 v0.0.0-20160222163258-40d72ab9641a // indirect
	github.com/juju/gojsonschema v0.0.0-20150312170016-e1ad140384f2
	github.com/juju/gomaasapi v0.0.0-20190826212825-0ab1eb636aba
	github.com/juju/jsonschema v0.0.0-20161102181919-a0ef8b74ebcf
	github.com/juju/jsonschema-gen v0.0.0-20200210135150-11507d258058
	github.com/juju/loggo v0.0.0-20190526231331-6e530bcce5d8
	github.com/juju/lru v0.0.0-20181205132344-305dec07bf2f // indirect
	github.com/juju/mutex v0.0.0-20180619145857-d21b13acf4bf
	github.com/juju/naturalsort v0.0.0-20180423034842-5b81707e882b
	github.com/juju/os v0.0.0-20200106011340-a3d14ebcfc13
	github.com/juju/packaging v0.0.0-20190830112352-cfc61ec3a3e9
	github.com/juju/persistent-cookiejar v0.0.0-20170428161559-d67418f14c93
	github.com/juju/proxy v0.0.0-20180523025733-5f8741c297b4
	github.com/juju/pubsub v0.0.0-20190419131051-c1f7536b9cc6
	github.com/juju/ratelimit v1.0.2-0.20191002062651-f60b32039441
	github.com/juju/replicaset v0.0.0-20190321104350-501ab59799b1
	github.com/juju/retry v0.0.0-20180821225755-9058e192b216
	github.com/juju/rfc v0.0.0-20180510112117-b058ad085c94
	github.com/juju/romulus v0.0.0-20191205211046-fd7cab26ac5f
	github.com/juju/rpcreflect v0.0.0-20190806165913-cca922e065df
	github.com/juju/schema v1.0.1-0.20190814234152-1f8aaeef0989
	github.com/juju/terms-client v1.0.2-0.20200108164349-d29f8efe0af6
	github.com/juju/testing v0.0.0-20191001232224-ce9dec17d28b
	github.com/juju/txn v0.0.0-20190416045819-5f348e78887d
	github.com/juju/usso v0.0.0-20160401104424-68a59c96c178 // indirect
	github.com/juju/utils v0.0.0-20200116185830-d40c2fe10647
	github.com/juju/version v0.0.0-20191219164919-81c1be00b9a6
	github.com/juju/webbrowser v0.0.0-20180907093207-efb9432b2bcb
	github.com/kr/pretty v0.1.0
	github.com/lestrrat/go-jspointer v0.0.0-20160229021354-f4881e611bdb // indirect
	github.com/lestrrat/go-jsref v0.0.0-20160601013240-e452c7b5801d // indirect
	github.com/lestrrat/go-jsschema v0.0.0-20160903131957-b09d7650b822 // indirect
	github.com/lestrrat/go-jsval v0.0.0-20161012045717-b1258a10419f // indirect
	github.com/lestrrat/go-pdebug v0.0.0-20160817063333-2e6eaaa5717f // indirect
	github.com/lestrrat/go-structinfo v0.0.0-20160308131105-f74c056fe41f // indirect
	github.com/lxc/lxd v0.0.0-20191002163551-3a5f63bc4959
	github.com/masterzen/azure-sdk-for-go v3.2.0-beta.0.20161014135628-ee4f0065d00c+incompatible // indirect
	github.com/masterzen/simplexml v0.0.0-20160608183007-4572e39b1ab9 // indirect
	github.com/masterzen/winrm v0.0.0-20161014151040-7a535cd943fc // indirect
	github.com/masterzen/xmlpath v0.0.0-20140218185901-13f4951698ad // indirect
	github.com/mattn/go-isatty v0.0.4
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/oracle/oci-go-sdk v5.7.0+incompatible
	github.com/pascaldekloe/goe v0.1.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.0.0
	github.com/prometheus/client_model v0.0.0-20190129233127-fd36f4220a90
	github.com/satori/go.uuid v1.2.0 // indirect
	github.com/smartystreets/goconvey v1.6.4 // indirect
	github.com/vmware/govmomi v0.21.1-0.20191008161538-40aebf13ba45
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	golang.org/x/crypto v0.0.0-20191206172530-e9b2fee46413
	golang.org/x/net v0.0.0-20200202094626-16171245cfb2
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sys v0.0.0-20191210023423-ac6580df4449
	google.golang.org/api v0.4.0
	gopkg.in/amz.v3 v3.0.0-20191122063134-7ba11a47c789
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15
	gopkg.in/goose.v2 v2.0.0-20200221151515-cc624cc0ae77
	gopkg.in/httprequest.v1 v1.2.0
	gopkg.in/ini.v1 v1.10.1
	gopkg.in/juju/blobstore.v2 v2.0.0-20160125023703-51fa6e26128d
	gopkg.in/juju/charm.v6 v6.0.0-20200220135436-5217f4ff9d6d
	gopkg.in/juju/charmrepo.v4 v4.1.3-0.20200210100342-4a4d9c6d92fd
	gopkg.in/juju/environschema.v1 v1.0.0
	gopkg.in/juju/idmclient.v1 v1.0.0-20180320161856-203d20774ce8
	gopkg.in/juju/names.v2 v2.0.0-20190813004204-e057c73bd1be // indirect
	gopkg.in/juju/names.v3 v3.0.0-20200131033104-139ecaca454c
	gopkg.in/juju/worker.v1 v1.0.0-20191018043616-19a698a7150f
	gopkg.in/macaroon-bakery.v2 v2.1.1-0.20190613120608-6734dc66fe81
	gopkg.in/macaroon.v2 v2.1.0
	gopkg.in/mgo.v2 v2.0.0-20190816093944-a6b53ec6cb22
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/natefinch/npipe.v2 v2.0.0-20160621034901-c1b8fa8bdcce
	gopkg.in/retry.v1 v1.0.2
	gopkg.in/robfig/cron.v2 v2.0.0-20150107220207-be2e0b0deed5 // indirect
	gopkg.in/tomb.v2 v2.0.0-20161208151619-d5d1b5820637
	gopkg.in/vmihailenco/msgpack.v2 v2.9.1 // indirect
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.0.0-20200131193051-d9adff57e763
	k8s.io/apiextensions-apiserver v0.0.0-20200131201446-6910daba737d
	k8s.io/apimachinery v0.17.4-beta.0
	k8s.io/client-go v0.0.0-20200131194156-19522ff28802
	k8s.io/utils v0.0.0-20200124190032-861946025e34 // indirect
	labix.org/v2/mgo v0.0.0-20140701140051-000000000287 // indirect
	launchpad.net/xmlpath v0.0.0-20130614043138-000000000004 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
)

replace github.com/altoros/gosigma => github.com/juju/gosigma v0.0.0-20170523021020-a27b59fe2be9

replace gopkg.in/mgo.v2 => github.com/juju/mgo v0.0.0-20190418114320-e9d4866cb7fc

replace github.com/hashicorp/raft => github.com/juju/raft v1.0.1-0.20190319034642-834fca2f9ffc

replace gopkg.in/yaml.v2 => github.com/juju/yaml v0.0.0-20181113141958-2025133c3826
