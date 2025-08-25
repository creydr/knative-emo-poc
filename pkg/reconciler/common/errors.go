package common

import "errors"

type NonRecoverableError struct {
	msg string
}

var _ error = NonRecoverableError{}

func (n NonRecoverableError) Error() string {
	if n.msg != "" {
		return n.msg
	}
	return "non recoverable error"
}

func IsNonRecoverableError(err error) bool {
	return errors.Is(err, NonRecoverableError{})
}

func NewNonRecoverableError(msg string) NonRecoverableError {
	return NonRecoverableError{
		msg: msg,
	}
}

type DeploymentsNotReadyError struct{}

var _ error = DeploymentsNotReadyError{}

// Error implements the Error() interface of error.
func (err DeploymentsNotReadyError) Error() string {
	return "deployments not ready"
}

// IsDeploymentsNotReadyError returns true if the given error is a DeploymentsNotReadyError.
func IsDeploymentsNotReadyError(err error) bool {
	return errors.Is(err, DeploymentsNotReadyError{})
}
