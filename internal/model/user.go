package model

import (
	"time"

	"gorm.io/gorm"
)

// User represents admin users and operators in the ISP system
type User struct {
	ID           string         `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	FullName     string         `gorm:"type:varchar(100);not null" json:"full_name"`
	Email        string         `gorm:"type:varchar(100);unique;not null" json:"email"`
	Phone        string         `gorm:"type:varchar(20)" json:"phone"`
	PasswordHash string         `gorm:"type:varchar(255);not null" json:"-"`
	Role         string         `gorm:"type:varchar(20);not null;default:'customer';check:role IN ('superadmin', 'admin', 'technician', 'sales_agent', 'customer')" json:"role"`
	IsActive     bool           `gorm:"type:boolean;default:true" json:"is_active"`
	LastLogin    *time.Time     `gorm:"type:timestamptz" json:"last_login"`
	LastIP       string         `gorm:"type:varchar(45)" json:"last_ip"`
	BearerKey    *string        `gorm:"type:varchar(255);unique" json:"-"`
	CreatedAt    time.Time      `gorm:"type:timestamptz;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"type:timestamptz;not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	CreatedCustomers []Customer `gorm:"foreignKey:CreatedBy" json:"-"`
	UpdatedCustomers []Customer `gorm:"foreignKey:UpdatedBy" json:"-"`
}

func (User) TableName() string {
	return "users"
}
