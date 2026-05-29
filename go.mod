module github.com/mobile-next/mobilecli

go 1.26.2

require (
	al.essio.dev/pkg/shellescape v1.5.1
	github.com/Masterminds/semver v1.5.0
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/danieljoos/wincred v1.2.2
	github.com/danielpaulus/go-ios v1.0.207-0.20260326100139-5d5f0d1129b8
	github.com/davecgh/go-spew v1.1.1
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572
	github.com/godbus/dbus/v5 v5.1.0
	github.com/google/btree v1.1.2
	github.com/google/pprof v0.0.0-20210407192527-94a9f03dee38
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/grandcat/zeroconf v1.0.0
	github.com/hashicorp/golang-lru/v2 v2.0.7
	github.com/inconshreveable/mousetrap v1.1.0
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0
	github.com/miekg/dns v1.1.57
	github.com/onsi/ginkgo/v2 v2.9.5
	github.com/pierrec/lz4 v2.6.1+incompatible
	github.com/pkg/errors v0.9.1
	github.com/pmezard/go-difflib v1.0.0
	github.com/quic-go/quic-go v0.40.1-0.20231203135336-87ef8ec48d55
	github.com/sevlyar/go-daemon v0.1.6
	github.com/sirupsen/logrus v1.9.3
	github.com/songgao/water v0.0.0-20200317203138-2b4b6d7c09d8
	github.com/spf13/cobra v1.9.1
	github.com/spf13/pflag v1.0.6
	github.com/stretchr/testify v1.10.0
	github.com/tadglines/go-pkgs v0.0.0-20210623144937-b983b20f54f9
	github.com/yapingcat/gomedia v0.0.0-20240906162731-17feea57090c
	github.com/zalando/go-keyring v0.2.6
	go.mozilla.org/pkcs7 v0.0.0-20210826202110-33d05740a352
	go.uber.org/mock v0.5.0
	golang.org/x/crypto v0.45.0
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842
	golang.org/x/mod v0.29.0
	golang.org/x/net v0.47.0
	golang.org/x/sync v0.18.0
	golang.org/x/sys v0.38.0
	golang.org/x/text v0.31.0
	golang.org/x/time v0.5.0
	golang.org/x/tools v0.38.0
	golang.zx2c4.com/wintun v0.0.0-20230126152724-0fa3db229ce2
	gopkg.in/ini.v1 v1.67.0
	gopkg.in/yaml.v3 v3.0.1
	gvisor.dev/gvisor v0.0.0-20240405191320-0878b34101b5
	howett.net/plist v1.0.1
	software.sslmate.com/src/go-pkcs12 v0.2.0
)

replace github.com/quic-go/quic-go => github.com/quic-go/quic-go v0.49.1

require github.com/kr/pretty v0.3.1 // indirect
