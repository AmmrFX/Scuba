package models

import (
	"time"
)

type DiveLog struct {
	Id        int       `json:"id"`
	DiverId   int       `json:"diverId"`
	Depth     float64   `json:"depth"`
	Timestamp time.Time `json:"timestamp"`
}
