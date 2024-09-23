module github.com/gofireflyio/k8s-collector

go 1.16

require (
	github.com/ido50/requests v1.2.0
	github.com/jgroeneveld/schema v1.0.0 // indirect
	github.com/jgroeneveld/trial v2.0.0+incompatible
	github.com/rs/zerolog v1.22.0
	github.com/thoas/go-funk v0.9.1
	golang.org/x/sync v0.0.0-20220722155255-886fb9371eb4
	gopkg.in/mgo.v2 v2.0.0-20190816093944-a6b53ec6cb22
	helm.sh/helm/v3 v3.10.3
	k8s.io/api v0.25.2
	k8s.io/apimachinery v0.25.2
	k8s.io/client-go v0.25.2
)
