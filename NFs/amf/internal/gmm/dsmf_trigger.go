package gmm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	amf_context "github.com/free5gc/amf/internal/context"
	"github.com/free5gc/amf/internal/logger"
	"github.com/free5gc/amf/pkg/factory"
	"github.com/free5gc/dataapi/pkg/api"
	"github.com/free5gc/openapi/models"
)

const (
	dsmfTriggerEventRegistrationRequest    = "registration_request"
	dsmfTriggerEventPDUSessionEstablishment = "pdu_session_establishment"
)

type dsmfTaskRequest struct {
	RequestID          string               `json:"requestId"`
	ResultMode         string               `json:"resultMode"`
	TransportProtocol  api.ProtocolType     `json:"transportProtocol"`
	PayloadProtocol    api.ProtocolType     `json:"payloadProtocol"`
	DataSource         api.DataSourceSpec   `json:"dataSource"`
	ProcessingSteps    []api.ProcessingStep `json:"processingSteps"`
	OutputSchema       string               `json:"outputSchema"`
	TaskTimeoutSeconds uint32               `json:"taskTimeoutSeconds"`
	Labels             map[string]string    `json:"labels"`
}

type dsmfTaskResponse struct {
	TaskID    string `json:"taskId"`
	State     string `json:"state"`
	ResultURI string `json:"resultUri"`
	Error     string `json:"error"`
}

func triggerDsmfDataTransfer(
	ue *amf_context.AmfUe,
	pduSessionID int32,
	dnn string,
	snssai models.Snssai,
	event string,
) {
	triggerCfg := factory.AmfConfig.GetDsmfTrigger()
	if triggerCfg == nil || !triggerCfg.Enable {
		return
	}
	if triggerCfg.TriggerEvent != "" && triggerCfg.TriggerEvent != event {
		return
	}

	supi := ue.Supi
	if supi == "" {
		supi = ue.Guti
	}

	request, endpoint, timeout, err := buildDsmfTriggerRequest(triggerCfg, supi, pduSessionID, dnn, snssai, event)
	if err != nil {
		logger.GmmLog.WithField(logger.FieldSupi, supi).Warnf("Skip DSMF trigger: %v", err)
		return
	}

	log := logger.GmmLog.WithField(logger.FieldSupi, supi).
		WithField("trigger_event", event).
		WithField("pdu_session_id", pduSessionID).
		WithField("dsmf_uri", endpoint)
	log.Infof("Dispatch DSMF trigger")

	body, err := json.Marshal(request)
	if err != nil {
		log.Errorf("Encode DSMF trigger request failed: %v", err)
		return
	}

	client := &http.Client{Timeout: timeout}
	httpReq, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		log.Errorf("Build DSMF trigger request failed: %v", err)
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		log.Errorf("AMF -> DSMF trigger failed: %v", err)
		return
	}
	defer resp.Body.Close()

	var taskResp dsmfTaskResponse
	if err := json.NewDecoder(resp.Body).Decode(&taskResp); err != nil {
		log.Errorf("Decode DSMF response failed: %v", err)
		return
	}

	if resp.StatusCode >= http.StatusBadRequest {
		log.Errorf("DSMF trigger rejected: status=%d taskId=%s error=%s", resp.StatusCode, taskResp.TaskID, taskResp.Error)
		return
	}

	log.Infof("Triggered DSMF data transfer: taskId=%s state=%s resultUri=%s", taskResp.TaskID, taskResp.State, taskResp.ResultURI)
}

func buildDsmfTriggerRequest(
	cfg *factory.DsmfTrigger,
	supi string,
	pduSessionID int32,
	dnn string,
	snssai models.Snssai,
	event string,
) (*dsmfTaskRequest, string, time.Duration, error) {
	if cfg.DataSource == nil {
		return nil, "", 0, fmt.Errorf("dsmf trigger data source is empty")
	}

	transport := api.ProtocolType(strings.ToUpper(cfg.TransportProtocol))
	if transport == "" {
		transport = api.ProtocolTypeHTTP2
	}
	payload := api.ProtocolType(strings.ToUpper(cfg.PayloadProtocol))
	if payload == "" {
		payload = api.ProtocolTypeJSON
	}

	steps := make([]api.ProcessingStep, 0, len(cfg.ProcessingSteps))
	for _, step := range cfg.ProcessingSteps {
		steps = append(steps, api.ProcessingStep{
			Name:       step.Name,
			Version:    step.Version,
			Parameters: kvPairs(step.Parameters),
		})
	}
	if len(steps) == 0 {
		steps = append(steps, api.ProcessingStep{Name: "passthrough", Version: "v1"})
	}

	labels := map[string]string{
		"triggeredBy":  "AMF",
		"triggerEvent": event,
		"ueSupi":       supi,
		"pduSessionId": strconv.Itoa(int(pduSessionID)),
		"dnn":          dnn,
		"snssaiSst":    strconv.Itoa(int(snssai.Sst)),
		"snssaiSd":     snssai.Sd,
	}
	for k, v := range cfg.Labels {
		labels[k] = v
	}

	requestID := fmt.Sprintf("amf-%s-pdu-%d-%d", sanitizeLabel(supi), pduSessionID, time.Now().UnixNano())
	request := &dsmfTaskRequest{
		RequestID:          requestID,
		ResultMode:         cfg.ResultMode,
		TransportProtocol:  transport,
		PayloadProtocol:    payload,
		DataSource:         api.DataSourceSpec{
			SourceID:         defaultString(cfg.DataSource.SourceID, requestID),
			SourceCategory:   api.DataSourceCategory(strings.ToUpper(cfg.DataSource.SourceCategory)),
			SourceScenario:   api.DataSourceScenario(strings.ToUpper(cfg.DataSource.SourceScenario)),
			SourceTypeDetail: cfg.DataSource.SourceTypeDetail,
			IngressEndpoint: api.Endpoint{
				Scheme: cfg.DataSource.IngressEndpoint.Scheme,
				Host:   cfg.DataSource.IngressEndpoint.Host,
				Port:   cfg.DataSource.IngressEndpoint.Port,
				Path:   cfg.DataSource.IngressEndpoint.Path,
			},
			Encoding:         cfg.DataSource.Encoding,
			SubscribedTopics: cfg.DataSource.SubscribedTopics,
			Attributes:       kvPairs(cfg.DataSource.Attributes),
		},
		ProcessingSteps:    steps,
		OutputSchema:       cfg.OutputSchema,
		TaskTimeoutSeconds: cfg.TaskTimeoutSeconds,
		Labels:             labels,
	}

	timeoutSeconds := cfg.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 10
	}

	return request, normalizeDsmfTasksEndpoint(cfg.Uri), time.Duration(timeoutSeconds) * time.Second, nil
}

func normalizeDsmfTasksEndpoint(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = "/ndsmf-data-service/v1/tasks"
		return parsed.String()
	}
	if strings.HasSuffix(parsed.Path, "/tasks") {
		return parsed.String()
	}
	parsed.Path = path.Join(parsed.Path, "/ndsmf-data-service/v1/tasks")
	return parsed.String()
}

func kvPairs(input map[string]string) []api.KvPair {
	if len(input) == 0 {
		return nil
	}
	pairs := make([]api.KvPair, 0, len(input))
	for key, value := range input {
		pairs = append(pairs, api.KvPair{Key: key, Value: value})
	}
	return pairs
}

func defaultString(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func sanitizeLabel(input string) string {
	replacer := strings.NewReplacer(" ", "-", ":", "-", "/", "-", ".", "-", "@", "-")
	return replacer.Replace(input)
}
