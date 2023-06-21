package models

import (
	"time"
)

type DiveLog struct {
	Id        int       `json:"id"`
	DiverId   int       `json:"diverId"`
	Depth     float64       `json:"deph"`
	Timestamp time.Time `json:"timestamp"`
}
