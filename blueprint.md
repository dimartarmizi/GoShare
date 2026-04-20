```md
# Blueprint: LAN File Sharing (Go + Wails)

## 1. Tujuan
Membangun aplikasi file sharing dalam jaringan lokal (LAN) dengan:
- Kecepatan tinggi (mendekati batas bandwidth LAN)
- Stabilitas tinggi (retry + resume)
- Minim konfigurasi (auto discovery)
- Distribusi mudah (single binary)

---

## 2. Arsitektur High-Level

```

+----------------------+
|      UI (Wails)      |
|  React / Vue / HTML  |
+----------+-----------+
|
| IPC (Wails binding)
|
+----------v-----------+

| Core Engine (Go)         |
| ------------------------ |
| - Transfer Manager       |
| - Chunk Engine           |
| - Connection Manager     |
| - Discovery Service      |
| - Integrity Checker      |
| +----------+-----------+ |

```
       |
       | TCP / UDP
       |
```

+----------v-----------+
|   Network Layer      |
| (LAN Communication)  |
+----------------------+

```

---

## 3. Modul Utama

### 3.1 Transfer Manager
Tanggung jawab:
- Mengatur lifecycle transfer
- Mengelola queue
- Monitoring progress

Fitur:
- Start / Pause / Resume / Cancel
- Multi-file transfer
- Prioritas transfer

---

### 3.2 Chunk Engine
Strategi:
- File dipecah menjadi chunk (1–4 MB)
- Chunk dikirim paralel

Struktur:
```

File
├── Chunk 1
├── Chunk 2
├── Chunk 3
└── ...

````

Fitur:
- Parallel upload/download
- Dynamic chunk size (opsional)
- Retry per chunk

---

### 3.3 Connection Manager
Tugas:
- Membuka koneksi TCP antar device
- Maintain session

Fitur:
- Keep-alive
- Timeout handling
- Reconnect logic

---

### 3.4 Discovery Service
Metode:
- UDP Broadcast
- mDNS (opsional)

Flow:
1. Broadcast: "SIAPA ADA?"
2. Device lain reply: "SAYA ADA + INFO"

Payload contoh:
```json
{
  "id": "device-123",
  "name": "Laptop A",
  "ip": "192.168.1.10",
  "port": 9000
}
````

---

### 3.5 Integrity Checker

* Hash per file (SHA256)
* Opsional: hash per chunk

Flow:

1. Sender kirim hash
2. Receiver validasi
3. Jika mismatch → retry

---

## 4. Protokol Transfer (Custom TCP)

### 4.1 Handshake

```
Client -> Server: HELLO
Server -> Client: OK + metadata
```

---

### 4.2 Metadata Exchange

```json
{
  "fileName": "video.mp4",
  "size": 104857600,
  "chunkSize": 1048576,
  "totalChunks": 100
}
```

---

### 4.3 Transfer Flow

```
[1] Request transfer
[2] Kirim metadata
[3] Kirim chunk paralel
[4] Receiver kirim ACK per chunk
[5] Retry jika gagal
[6] Final checksum
```

---

### 4.4 Chunk Packet Format

```
[HEADER]
- fileID
- chunkIndex
- chunkSize

[BODY]
- binary data
```

---

## 5. Strategi Performa

### 5.1 Parallelism

* Gunakan goroutine
* Batasi concurrency (misal 5–10 koneksi)

### 5.2 Buffering

* `bufio.Reader` / `bufio.Writer`
* Hindari read kecil

### 5.3 Zero-copy

* Gunakan `io.Copy`

---

## 6. Fault Tolerance

### 6.1 Retry Mechanism

* Retry per chunk
* Max retry (misal 5x)

### 6.2 Resume Transfer

* Simpan:

  * chunk selesai
  * offset file

### 6.3 Timeout Handling

* Detect koneksi mati
* Reconnect otomatis

---

## 7. UI (Wails)

### Halaman utama:

* Device list (auto discover)
* Drag & drop file
* Transfer progress

### Komponen:

* Progress bar
* Speed indicator (MB/s)
* Status (sending, retrying, completed)

---

## 8. Flow UX

### Kirim file:

1. Buka app
2. Device muncul otomatis
3. Drag file ke device
4. Transfer langsung jalan

---

### Terima file:

1. Notifikasi masuk
2. Accept / auto accept
3. Progress tampil

---

## 9. Distribusi

### Mode:

* Single binary (portable)

### Strategi:

* Kirim via:

  * USB
  * Download dari device lain (mini HTTP server)

---

## 10. Hybrid Mode (Opsional, Highly Recommended)

### Tujuan:

Mengalahkan PairDrop dalam UX awal

### Cara:

* Jalankan HTTP server kecil di Go
* Device lain buka browser:

  ```
  http://192.168.1.x:8080
  ```
* Halaman web:

  * Bisa kirim file langsung (mode web)
  * Ada tombol download app (native mode)

---

## 11. Security (Opsional)

* TLS di LAN
* Token pairing
* Whitelist device

---

## 12. Struktur Project

```
project-root/
├── main.go
├── app/
│   ├── transfer/
│   ├── chunk/
│   ├── network/
│   ├── discovery/
│   └── integrity/
├── frontend/
│   ├── src/
│   └── build/
├── internal/
│   └── utils/
└── assets/
```

---

## 13. Roadmap Implementasi

### Phase 1 (MVP)

* TCP transfer basic
* Single file
* No chunking

### Phase 2

* Chunking + parallel
* Progress UI

### Phase 3

* Discovery (UDP)
* Multi-device

### Phase 4

* Resume + retry
* Integrity check

### Phase 5

* Hybrid web mode
* UX polishing

---

## 14. Target Performa

* LAN speed: 50–110 MB/s (Gigabit)
* Latency: < 10ms (local)
* Failure recovery: < 3 detik reconnect

---

## 15. Risiko & Tantangan

* Firewall blocking
* WiFi packet loss
* OS permission (file access)
* Cross-platform behavior

---

## 16. Kesimpulan

Dengan desain ini:

* Performa bisa melebihi PairDrop
* Stabilitas lebih tinggi
* UX bisa mendekati zero-config

Kunci keberhasilan:

* Discovery cepat
* Transfer paralel
* Retry + resume solid
* UX sederhana

```
```
