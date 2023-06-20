package models

import (
	"encoding/json"
)

type Diver struct {
	Id       int             `json:"id"`
	Name     string          `json:"name"`
	DiverEqp json.RawMessage `json:"diverEqp"`
}
