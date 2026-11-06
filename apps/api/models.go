package main

import "time"

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Email        string    `gorm:"uniqueIndex;not null;size:255" json:"email"`
	PasswordHash string    `gorm:"not null" json:"-"` // json:"-" → never sent to clients
	CreatedAt    time.Time `json:"created_at"`
}

type Submission struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	Language  string    `gorm:"size:10;not null;default:python" json:"language"`
	Code      string    `gorm:"type:text;not null" json:"code"`
	Stdout    string    `gorm:"type:text" json:"stdout"`
	Stderr    string    `gorm:"type:text" json:"stderr"`
	ExitCode  int       `json:"exit_code"`
	TimedOut  bool      `json:"timed_out"`
	CreatedAt time.Time `json:"created_at"`
}
