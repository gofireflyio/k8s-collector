package common

import "time"

type CollectionStats struct {
	CollectionTime time.Duration `json:"collectionTime"`
	RedactionTime  time.Duration `json:"redactionTime"`
}
