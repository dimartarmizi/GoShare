# GoShare (MVP)

Implementasi awal dari blueprint LAN file sharing (Go + Wails-ready architecture).

## Yang Sudah Diimplementasikan

- Core structure sesuai blueprint:
  - `app/transfer`
  - `app/chunk`
  - `app/network`
  - `app/discovery`
  - `app/integrity`
- TCP transfer dengan chunk engine (Phase 2 baseline)
  - Handshake: `HELLO` -> `OK`
  - Metadata exchange: `META {...}`
  - Resume negotiation: `READY <offset>`
  - Chunk framing: `CHUNK <index> <size>`
  - ACK/NACK per chunk (`ACK <index>` / `NACK <index>`)
  - Retry otomatis per chunk (maks 5x)
  - Final checksum SHA256: `DONE <hash>`
- UDP discovery service (broadcast)
  - Request: `GOSHARE_DISCOVER`
  - Reply: `GOSHARE_HERE <json device info>`
- Transfer manager untuk lifecycle task (queued/running/completed/failed/canceled)
- Integrity checker (SHA256)
- Backend facade untuk binding UI/Wails: `app/backend.go`

## Jalankan UI Desktop (Wails)

Default entrypoint sekarang adalah desktop app. Jadi:

```bash
go run -tags dev .
```

akan langsung membuka tampilan aplikasi.

Untuk mode production (tanpa popup error build tags), gunakan:

```bash
go run -tags production .
```

## Build Binary Desktop

```bash
go build -tags production -o goshare.exe .
```

Lalu jalankan `goshare.exe` dan UI akan muncul.

## Build Semua Package

```bash
go build -tags production ./...
```

## Opsi Resmi Wails CLI

Jika memakai CLI Wails (disarankan):

```bash
wails dev
wails build
```

## Mode CLI (Legacy)

Mode CLI lama dipindah ke `cmd/cli`:

```bash
go run ./cmd/cli -mode server -listen :9000 -save-dir ./received -name "Laptop A"
```

## Menjalankan

### 1. Jalankan penerima (server)

```bash
go run ./cmd/cli -mode server -listen :9000 -save-dir ./received -name "Laptop A"
```

### 2. Discovery device di LAN

```bash
go run ./cmd/cli -mode discover -discovery-port 9999 -timeout 3s
```

### 3. Kirim file

```bash
go run ./cmd/cli -mode send -target 192.168.1.10:9000 -file ./contoh.zip
```

## Catatan

- Implementasi ini fokus ke baseline MVP yang stabil dan modular.
- Chunking, ACK/NACK, dan retry per chunk sudah aktif.
- Resume offset antar sesi sudah aktif menggunakan file sementara `.part`.
- Chunk parallel multi-connection dan UI Wails penuh belum diaktifkan, tapi API backend dan pondasi modulnya sudah disiapkan untuk phase berikutnya.
