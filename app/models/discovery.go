package models

type DiscoveryMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	IP        string `json:"ip"`
	Port      int    `json:"port"`
	Timestamp int64  `json:"timestamp"`
}
