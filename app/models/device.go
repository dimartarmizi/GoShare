package models

type Device struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	IsOnline bool   `json:"isOnline"`
}
