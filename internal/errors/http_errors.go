package errors

import "net/http"

func InvalidArg(arg string) error {
	return Newf(nil, http.StatusBadRequest, "invalid argument: %s", arg)
}

func BadRequest(message string) error {
	return Newf(nil, http.StatusBadRequest, "%s", message)
}

func Unauthorized(message string) error {
	return Newf(nil, http.StatusUnauthorized, "%s", message)
}

func InternalServerError(message string) error {
	return Newf(nil, http.StatusInternalServerError, "%s", message)
}

func HTTPShutDown(cause error) error {
	return Newf(cause, http.StatusInternalServerError, "http server shut down")
}
