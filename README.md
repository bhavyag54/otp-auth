# OTP Authentication Service

A lightweight Go microservice that delivers SMS-based One-Time Passwords (OTP) using the Gin web framework and Twilio.

## Features

- Phone number-based authentication via SMS OTP
- Cryptographically secure OTP generation with a deterministic fallback
- In-memory OTP cache with 5-minute expiry and automatic cleanup
- Docker & Docker Compose support for easy deployment

## Prerequisites

- Go 1.21 or newer (for local development)
- Twilio account credentials:
  - **TWILLIO_SID** – Account SID
  - **TWILLIO_AUTH_TOKEN** – Auth token
  - **TWILLIO_PHONE** – Verified outbound phone number

## Environment Variables

Create a `.env` file in the project root:

```env
# Twilio
TWILLIO_SID=your_twilio_account_sid
TWILLIO_AUTH_TOKEN=your_twilio_auth_token
TWILLIO_PHONE=+1234567890

# Application
ENV=development
GIN_MODE=debug
```

## API

### 1. Generate OTP

```http
POST /otp
Content-Type: application/json

{
  "phone": "+1234567890"
}
```
Response – **200 OK**
```json
{ "message": "OTP sent successfully" }
```

### 2. Validate OTP

```http
POST /otp/validate
Content-Type: application/json

{
  "phone": "+1234567890",
  "otp": "1234"
}
```
Possible responses

| Status | Body | Description |
| ------ | ---- | ----------- |
| 200 | `{ "valid": true }` | OTP is correct |
| 400 | `{ "valid": false, "error": "OTP has expired" \| "OTP is incorrect" }` | Expired or wrong OTP |
| 404 | `{ "valid": false, "error": "OTP not found" }` | No OTP issued for the phone |
| 500 | `{ "valid": false, "error": "Internal server error" }` | Unexpected failure |

## Running the Service

### Local Development

```bash
go mod download
go run auth.go   # service listens on :8000
```

### Docker

```bash
docker-compose up --build  # maps :8000 -> :8000 by default
# or
docker build -t auth-service .
docker run -p 8000:8000 --env-file .env auth-service
```

## License

MIT 