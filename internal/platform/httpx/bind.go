// This file binds JSON request bodies for HTTP handlers.
package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
)

func BindJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return errors.New("request body is required")
	}
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}
