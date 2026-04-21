package models

import "time"

const (
	TransferStatusPending    = "pending"
	TransferStatusInProgress = "in_progress"
	TransferStatusPaused     = "paused"
	TransferStatusCompleted  = "completed"
	TransferStatusFailed     = "failed"
	TransferStatusCanceled   = "canceled"
	TransferStatusRejected   = "rejected"
)

const (
	TransferDirectionIncoming = "incoming"
	TransferDirectionOutgoing = "outgoing"
)

type FileMeta struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type Transfer struct {
	ID               string     `json:"id"`
	PeerID           string     `json:"peerId"`
	PeerName         string     `json:"peerName"`
	Direction        string     `json:"direction"`
	DirectionLabel   string     `json:"directionLabel"`
	Status           string     `json:"status"`
	StatusLabel      string     `json:"statusLabel"`
	Files            []FileMeta `json:"files"`
	TotalBytes       int64      `json:"totalBytes"`
	TransferredBytes int64      `json:"transferredBytes"`
	Progress         float64    `json:"progress"`
	Error            string     `json:"error,omitempty"`
	StartedAt        time.Time  `json:"startedAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}
