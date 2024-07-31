module github.com/gofireflyio/k8s-collector

go 1.16

require (
	github.com/ido50/requests v1.6.0
	github.com/infralight/redactor/pkg v0.0.19
	github.com/jgroeneveld/trial v2.0.0+incompatible
	github.com/rs/zerolog v1.30.0
	github.com/spf13/viper v1.10.1
	github.com/thoas/go-funk v0.9.2
	github.com/zricethezav/gitleaks/v8 v8.2.7
	golang.org/x/sync v0.1.0
	gopkg.in/mgo.v2 v2.0.0-20190816093944-a6b53ec6cb22
	helm.sh/helm/v3 v3.9.1
	k8s.io/api v0.24.3
	k8s.io/apimachinery v0.24.3
	k8s.io/client-go v0.24.3
)
