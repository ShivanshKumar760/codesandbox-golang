package main

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var jwtSecret = []byte(os.Getenv("JWT_SECRET"))

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func register(db *gorm.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		var in credentials
		if err := c.Bind().Body(&in); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid json")
		}

		in.Email = strings.ToLower(strings.TrimSpace(in.Email))
		if !strings.Contains(in.Email, "@") || len(in.Password) < 8 {
			return fiber.NewError(fiber.StatusBadRequest, "valid email and password(8+ chars) required")
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}

		user := User{Email: in.Email, PasswordHash: string(hash)}

		if err := db.Create(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(err.Error(), "duplicate") {
				return fiber.NewError(fiber.StatusConflict, "email already registered")
			}
			return err
		}

		return c.Status(fiber.StatusCreated).JSON(user)
	}
}

func login(db *gorm.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		var in credentials
		if err := c.Bind().Body(&in); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid json")
		}

		var user User
		err := db.Where("email = ?", strings.ToLower(strings.TrimSpace(in.Email))).First(&user).Error
		if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.Password)) != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "invalid email or password")
		}

		claims := jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(uint64(user.ID), 10),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		}

		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(jwtSecret)
		if err != nil {
			return err
		}
		return c.JSON(fiber.Map{"token": token})
	}
}

func authRequired(c fiber.Ctx) error {
	header := c.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return fiber.NewError(fiber.StatusUnauthorized, "missing bearer token")
	}

	raw := strings.TrimPrefix(header, "Bearer ")
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) { return jwtSecret, nil }, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !token.Valid {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid or expired token")
	}

	id, err := strconv.ParseUint(claims.Subject, 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "bad token subject")
	}
	c.Locals("userID", uint(id))
	return c.Next()
}

func me(db *gorm.DB) fiber.Handler {
	return func(c fiber.Ctx) error {
		var user User
		if err := db.First(&user, fiber.Locals[uint](c, "userID")).Error; err != nil {
			return fiber.NewError(fiber.StatusNotFound, "user not found")
		}
		return c.JSON(user)
	}
}
