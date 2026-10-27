package model

import (
	"time"
)

type Trip struct {
	ID             string
	UserID         string
	DriverID       string
	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64
	Price          int64
	Status         string
	StartedAt      time.Time
	FinishedAt     *time.Time
}

type TripStatusHistory struct {
	TripID     string
	FromStatus *string
	ToStatus   string
	Reason     *string
}
