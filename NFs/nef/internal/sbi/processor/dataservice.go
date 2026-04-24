package processor

import (
	"bytes"
	"io"
	"net/http"
)

func (p *Processor) ForwardDataServiceTask(body []byte, contentType string) HandlerResponse {
	dsmfURI := p.Config().DsmfUri()
	if dsmfURI == "" {
		return HandlerResponse{
			Status: http.StatusBadGateway,
			Body:   []byte(`{"error":"dsmfUri is not configured"}`),
		}
	}

	if contentType == "" {
		contentType = "application/json"
	}

	req, err := http.NewRequest(http.MethodPost, dsmfURI+"/ndsmf-data-service/v1/tasks", bytes.NewReader(body))
	if err != nil {
		return HandlerResponse{
			Status: http.StatusInternalServerError,
			Body:   []byte(`{"error":"failed to create DSMF request"}`),
		}
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return HandlerResponse{
			Status: http.StatusBadGateway,
			Body:   []byte(`{"error":"failed to call DSMF"}`),
		}
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return HandlerResponse{
			Status: http.StatusBadGateway,
			Body:   []byte(`{"error":"failed to read DSMF response"}`),
		}
	}

	return HandlerResponse{
		Status:  resp.StatusCode,
		Headers: resp.Header,
		Body:    payload,
	}
}
