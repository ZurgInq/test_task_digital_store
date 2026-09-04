package user

import (
	"app/models/currency"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model

	Balance  currency.Amount
	Currency currency.Currency
}
