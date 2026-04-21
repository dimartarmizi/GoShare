# GoShare

GoShare adalah aplikasi desktop berbasis Go + Wails untuk berbagi file antar perangkat dalam satu jaringan lokal (WiFi/LAN) tanpa internet.

## Yang Sudah Diimplementasikan (MVP)

- Auto-discovery device realtime (UDP broadcast + heartbeat 1 detik)
- Device status online/offline (offline jika > 3 detik tidak ada heartbeat)
- Transfer file via TCP dengan mekanisme chunk 64KB
- Progress transfer realtime
- Incoming transfer dengan Accept / Reject
- Kontrol transfer outgoing: Pause / Resume / Cancel
- Multi-connection handling dengan batas concurrency
- UI desktop Wails (device list, file picker, transfer list)

## Struktur Proyek

- main.go
- app.go
- app/discovery
- app/transfer
- app/connection
- app/models
- app/utils
- frontend/dist
- wails.json

## Menjalankan

1. Pastikan Go sudah terpasang.
2. Jalankan aplikasi:

```bash
go run -tags dev .
```

Catatan: aplikasi Wails butuh build tags. Jika tanpa tag, akan muncul error "Wails applications will not build without the correct build tags".

## Build Portable Binary

Build langsung via Go:

```bash
go build -tags production -ldflags="-s -w -H=windowsgui" -o GoShare.exe .
```

Output binary berada di root folder proyek.

## Port & Protocol

- UDP discovery: 9999
- TCP transfer: 9000
- Control plane: JSON frames
- Data plane: binary chunk frames

## Catatan

- Folder penerimaan default: ~/Downloads/GoShare
- MVP saat ini fokus file transfer (folder transfer belum aktif)
- Resume after disconnect masih masuk roadmap v2
