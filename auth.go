package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	crand "crypto/rand"
	"encoding/binary"

	mathrand "math/rand"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	twilio "github.com/twilio/twilio-go"
	openapi "github.com/twilio/twilio-go/rest/api/v2010"

	"auth-service/backend"
)

// OTPRequest represents the request body for OTP generation
type OTPRequest struct {
	Phone string `json:"phone" binding:"required"`
}

// ValidateOTPRequest represents the request body for OTP validation
type ValidateOTPRequest struct {
	Otp   string `json:"otp"   binding:"required"`
	Phone string `json:"phone" binding:"required"`
}

var (
	otpCache     backend.OTPCache
	twilioClient *twilio.RestClient
	fallbackRand *mathrand.Rand
)

func init() {
	// Load environment variables
	if os.Getenv("ENV") != "production" {
		if err := godotenv.Load(); err != nil {
			log.Println("No .env file found")
		}
	}

	// Initialize in-memory OTP cache
	otpCache = backend.NewMemoryOTPCache()

	twilioClient = twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: os.Getenv("TWILLIO_SID"),
		Password: os.Getenv("TWILLIO_AUTH_TOKEN"),
	})

	// Initialize local math/rand generator (used only as crypto fallback)
	fallbackRand = mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
}

func main() {
	r := gin.Default()

	// Public routes
	r.POST("/otp", generateOTP)
	r.POST("/otp/validate", validateOTP)

	r.Run(":8000")
}

func generateOTP(c *gin.Context) {
	var req OTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	otp := generateSecureOTP()

	params := &openapi.CreateMessageParams{}
	// Ensure phone number is in E.164 format
	formattedPhone := req.Phone
	if !strings.HasPrefix(formattedPhone, "+") {
		formattedPhone = "+" + formattedPhone
	}
	params.SetTo(formattedPhone)
	params.SetFrom(os.Getenv("TWILLIO_PHONE"))
	params.SetBody("Your OTP is: " + otp)

	_, err := twilioClient.Api.CreateMessage(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send OTP: " + err.Error()})
		return
	}

	// Store OTP in cache
	if err := otpCache.SetOTP(ctx, req.Phone, otp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store OTP"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "OTP sent successfully"})
}

func validateOTP(c *gin.Context) {
	var req ValidateOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"valid": false, "error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	// Retrieve OTP from cache
	storedOTP, err := otpCache.GetOTP(ctx, req.Phone)
	if err != nil {
		switch err {
		case backend.ErrOTPExpired:
			c.JSON(http.StatusBadRequest, gin.H{"valid": false, "error": "OTP has expired"})
		case backend.ErrOTPNotFound:
			c.JSON(http.StatusNotFound, gin.H{"valid": false, "error": "OTP not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"valid": false, "error": "Internal server error"})
		}
		return
	}

	if storedOTP != req.Otp {
		c.JSON(http.StatusOK, gin.H{"valid": false, "error": "OTP is incorrect"})
		return
	}

	// OTP is valid; delete it to enforce one-time usage
	if err := otpCache.DeleteOTP(ctx, req.Phone); err != nil {
		log.Printf("Failed to delete OTP from cache: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"valid": true})
}

func generateSecureOTP() string {
	// Generates a 4-digit OTP using crypto/rand for better security
	var b [2]byte
	if _, err := crand.Read(b[:]); err != nil {
		// Fallback to math/rand if crypto/rand fails
		return strconv.Itoa(1000 + fallbackRand.Intn(9000))
	}
	// Convert the random bytes to uint16 and limit to 4-digit range
	num := binary.BigEndian.Uint16(b[:]) % 9000
	return strconv.Itoa(int(num) + 1000)
}
