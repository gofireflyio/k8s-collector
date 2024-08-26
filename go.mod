module github.com/gofireflyio/k8s-collector

go 1.16

require (
	github.com/ido50/requests v1.6.0
	github.com/infralight/redactor/pkg v0.0.20-0.20240826132026-931bdc9e0778
	github.com/jgroeneveld/trial v2.0.0+incompatible
	github.com/rs/zerolog v1.30.0
	github.com/thoas/go-funk v0.9.2
	golang.org/x/sync v0.1.0
	gopkg.in/mgo.v2 v2.0.0-20190816093944-a6b53ec6cb22
	helm.sh/helm/v3 v3.10.3
	k8s.io/api v0.25.2
	k8s.io/apimachinery v0.25.2
	k8s.io/client-go v0.25.2
)
