package storage

import "errors"

var ErrCodeNotFound = errors.New("item not found in storage")
var ErrItemWithIDAlreadyExists = errors.New("voting category already exists")
