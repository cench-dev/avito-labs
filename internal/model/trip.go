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

type TripPosition struct {
	ID         int64
	TripID     string
	Latitude   float64
	Longitude  float64
	RecordedAt time.Time
	CreatedAt  time.Time
}
