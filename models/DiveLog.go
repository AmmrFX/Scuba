package models

import (
	"time"
)

type DiveLog struct {
	Id        int       `json:"id"`
	DiverId   int       `json:"diverId"`
	Depth     int       `json:"depth"`
	Timestamp time.Time `json:"timestamp"`
}
