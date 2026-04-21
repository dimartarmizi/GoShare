package models

import "time"

type Device struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	IP         string    `json:"ip"`
	Port       int       `json:"port"`
	LastSeen   time.Time `json:"lastSeen"`
	LastOnline time.Time `json:"lastOnline"`
	IsOnline   bool      `json:"isOnline"`
	Latency    int       `json:"latency"`
}
