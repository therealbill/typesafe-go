package typesafe

import "encoding/json"

// ModelMetadata describes one model available to the account.
type ModelMetadata struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// ReleaseDate is the timestamp string reported by the API.
	ReleaseDate string `json:"release_date"`
}

// ListModelsResponse is the decoded result of a ListModels call.
type ListModelsResponse struct {
	Models    []ModelMetadata `json:"models"`
	RequestID string          `json:"request_id,omitempty"`
	Raw       *RawResponse    `json:"-"`
}

func decodeModels(body []byte) (*ListModelsResponse, error) {
	var res ListModelsResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fieldErr("", err)
	}
	return &res, nil
}
