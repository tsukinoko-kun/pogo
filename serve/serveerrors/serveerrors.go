package serveerrors

import "errors"

var (
	ErrPushToChangeWithChild = errors.New("pushing to a change that has children is not allowed")
	ErrPushToChangeNotOwned  = errors.New("pushing to a change that was created by another user or device is not allowed")
)
