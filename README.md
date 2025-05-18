# OTP-Based Authentication Service

A secure authentication service built with Go, Gin, and Twilio that implements OTP (One-Time Password) based authentication using SMS.

## Features

- Phone number-based authentication using OTP
- Secure token-based session management
- JWT-based access tokens
- Refresh token mechanism
- Cookie-based token storage
- PostgreSQL database integration
- Docker support

## Prerequisites

- Go 1.24 or higher
- PostgreSQL database
- Twilio account with:
  - Account SID
  - Auth Token
  - Twilio Phone Number

## Environment Variables

Create a `.env` file in the root directory with the following variables:

```env
# Database Configuration
DB_HOST=localhost
DB_USER=your_db_user
DB_PASSWORD=your_db_password
DB_NAME=your_db_name
DB_PORT=5432

# JWT Configuration
JWT_SECRET=your_jwt_secret_key

# Twilio Configuration
TWILLIO_SID=your_twilio_account_sid
TWILLIO_AUTH_TOKEN=your_twilio_auth_token
TWILLIO_PHONE=your_twilio_phone_number

# Environment
ENV=development
```

## API Endpoints

### Public Endpoints

1. **Generate OTP**
   ```http
   POST /otp
   Content-Type: application/json

   {
     "phone": "1234567890"
   }
   ```
   - Sends an OTP to the provided phone number
   - Phone number should be in E.164 format (e.g., +1234567890)

2. **Login with OTP**
   ```http
   POST /login
   Content-Type: application/json

   {
     "otp": "1234"
   }
   ```
   - Verifies the OTP and issues access and refresh tokens
   - Requires a valid phone cookie from the OTP generation step

### Protected Endpoints

All protected endpoints require a valid access token in the cookie.

1. **Logout**
   ```http
   POST /logout
   ```
   - Invalidates the current session
   - Clears access and refresh tokens

2. **Refresh Token**
   ```http
   POST /refresh
   ```
   - Issues new access and refresh tokens
   - Requires a valid refresh token

3. **Verify Token**
   ```http
   GET /verify
   ```
   - Verifies the current access token
   - Returns the user ID if valid

## Running the Service

### Local Development

1. Install dependencies:
   ```bash
   go mod download
   ```

2. Run the service:
   ```bash
   go run auth.go
   ```

### Docker

#### Using Docker Compose (Recommended)

1. Make sure your `.env` file is properly configured with all required variables

2. Run the service:
   ```bash
   docker-compose up --build
   ```

   To run in detached mode:
   ```bash
   docker-compose up -d --build
   ```

3. To stop the service:
   ```bash
   docker-compose down
   ```

#### Using Docker Directly

1. Build the Docker image:
   ```bash
   docker build -t auth-service .
   ```

2. Run the container:
   ```bash
   docker run -p 8080:8080 --env-file .env auth-service
   ```

## Security Features

- OTP expiration
- Secure cookie settings
- JWT token expiration
- Refresh token rotation
- Phone number verification
- HTTP-only cookies
- Secure password hashing

## Error Handling

The service returns appropriate HTTP status codes and error messages:

- 400: Bad Request
- 401: Unauthorized
- 500: Internal Server Error

## License

This project is licensed under the MIT License - see the LICENSE file for details. 