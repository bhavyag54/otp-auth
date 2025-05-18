package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"strconv"

	"math/rand"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	twilio "github.com/twilio/twilio-go"
	openapi "github.com/twilio/twilio-go/rest/api/v2010"
)

// User represents the user model
type User struct {
	gorm.Model
	Phone        string `gorm:"uniqueIndex"`
	Email        string `gorm:"uniqueIndex"`
	OTP          string
	RefreshToken string
	IsVerified   bool `gorm:"default:false"`
}

// TokenResponse represents the response structure for token endpoints
type TokenResponse struct {
	Message string `json:"message"`
}

// LoginRequest represents the login request structure
type OTPRequest struct {
	Phone string `json:"phone" binding:"required"`
}

type LoginRequest struct {
	Otp   string `json:"otp" binding:"required"`
}

var (
	db     *gorm.DB
	jwtKey = []byte(os.Getenv("JWT_SECRET"))
)

func init() {
	// Load environment variables

	if os.Getenv("ENV") != "production" {
		if err := godotenv.Load(); err != nil {
			log.Println("No .env file found")
		}
	}

	// Initialize database connection
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
	)

	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Auto migrate the schema
	db.AutoMigrate(&User{})
}

func main() {
	r := gin.Default()

	// Public routes
	r.POST("/otp", generateOTP)
	r.POST("/login", login)

	// Protected routes
	auth := r.Group("/")
	auth.Use(authMiddleware())
	{
		auth.POST("/logout", logout)
		auth.POST("/refresh", refreshToken)
		auth.GET("/verify", verifyToken)
	}

	r.Run(":8080")
}

func generateOTP(c *gin.Context) {
	var req OTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user User
	auth_err := db.Where("phone = ?", req.Phone).First(&user).Error
	fmt.Println("Error: ", auth_err)
	fmt.Println("User: ", user)
	if auth_err != nil {
		user.Phone = req.Phone
		user.OTP = ""
		user.IsVerified = false
		user.RefreshToken = ""
		db.Create(&user)
	}

	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: os.Getenv("TWILLIO_SID"),
		Password: os.Getenv("TWILLIO_AUTH_TOKEN"),
	})

	otp := strconv.Itoa(rand.Intn(9000) + 1000)
	
	params := &openapi.CreateMessageParams{}
	// Ensure phone number is in E.164 format
	formattedPhone := req.Phone
	if !strings.HasPrefix(formattedPhone, "+") {
		formattedPhone = "+" + formattedPhone
	}
	params.SetTo(formattedPhone)
	params.SetFrom(os.Getenv("TWILLIO_PHONE"))
    params.SetBody("Your OTP is: " + otp)

	_, err := client.Api.CreateMessage(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send OTP: " + err.Error()})
		return
	}

	user.OTP = otp
	db.Save(&user)

	c.SetCookie("phone", req.Phone, 0, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{"message": "OTP sent successfully"})
}

func login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	phone, err := c.Cookie("phone")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	var user User
	if err := db.Where("phone = ?", phone).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if user.OTP == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if user.OTP != req.Otp {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid OTP"})
		return
	}

	user.IsVerified = true

	// Generate tokens
	accessToken, err := generateAccessToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	refreshToken := uuid.New().String()

	// Update user's refresh token
	user.RefreshToken = refreshToken
	user.OTP = ""
	db.Save(&user)

	// Set access token cookie
	c.SetCookie(
		"access_token",
		accessToken,
		24*60*60, // 24 hours
		"/",
		"",
		false, // secure
		true, // httpOnly
	)

	// Set refresh token cookie
	c.SetCookie(
		"refresh_token",
		refreshToken,
		30*24*60*60, // 30 days
		"/",
		"",
		false, // secure
		true, // httpOnly
	)

	c.JSON(http.StatusOK, TokenResponse{
		Message: "Login successful",
	})
}

func logout(c *gin.Context) {
	userID := c.GetUint("user_id")

	var user User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find user"})
		return
	}

	// Clear refresh token
	user.RefreshToken = ""
	db.Save(&user)

	// Clear cookies
	c.SetCookie("access_token", "", -1, "/", "", true, true)
	c.SetCookie("refresh_token", "", -1, "/", "", true, true)

	c.JSON(http.StatusOK, TokenResponse{
		Message: "Logged out successfully",
	})
}

func refreshToken(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Refresh token required"})
		return
	}

	var user User
	if err := db.Where("refresh_token = ?", refreshToken).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	// Generate new tokens
	accessToken, err := generateAccessToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	newRefreshToken := uuid.New().String()
	user.RefreshToken = newRefreshToken
	db.Save(&user)

	// Set new access token cookie
	c.SetCookie(
		"access_token",
		accessToken,
		24*60*60, // 24 hours
		"/",
		"",
		true, // secure
		true, // httpOnly
	)

	// Set new refresh token cookie
	c.SetCookie(
		"refresh_token",
		newRefreshToken,
		30*24*60*60, // 30 days
		"/",
		"",
		true, // secure
		true, // httpOnly
	)

	c.JSON(http.StatusOK, TokenResponse{
		Message: "Tokens refreshed successfully",
	})
}

func verifyToken(c *gin.Context) {
	// If we reach here, the token is already verified by the middleware
	userID := c.GetUint("user_id")
	c.JSON(http.StatusOK, gin.H{"user_id": userID})
}

func generateAccessToken(userID uint) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})

	return token.SignedString(jwtKey)
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := c.Cookie("access_token")
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization cookie required"})
			c.Abort()
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return jwtKey, nil
		})

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			userID := uint(claims["user_id"].(float64))
			c.Set("user_id", userID)
			c.Next()
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}
	}
}
