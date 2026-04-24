package api

type ProtocolType string

const (
	ProtocolTypeUnspecified ProtocolType = "UNSPECIFIED"
	ProtocolTypeRDMA        ProtocolType = "RDMA"
	ProtocolTypeProtobuf    ProtocolType = "PROTOBUF"
	ProtocolTypeQUIC        ProtocolType = "QUIC"
	ProtocolTypeJSON        ProtocolType = "JSON"
	ProtocolTypeHTTP2       ProtocolType = "HTTP2"
	ProtocolTypeHTTP3       ProtocolType = "HTTP3"
)

type ProcessingState string

const (
	ProcessingStateUnspecified      ProcessingState = "UNSPECIFIED"
	ProcessingStateAccepted         ProcessingState = "ACCEPTED"
	ProcessingStateReceivingSource  ProcessingState = "RECEIVING_SOURCE"
	ProcessingStatePreprocessing    ProcessingState = "PREPROCESSING"
	ProcessingStateProcessing       ProcessingState = "PROCESSING"
	ProcessingStateDelivering       ProcessingState = "DELIVERING"
	ProcessingStateCompleted        ProcessingState = "COMPLETED"
	ProcessingStateFailed           ProcessingState = "FAILED"
)

type StorageState string

const (
	StorageStateUnspecified   StorageState = "UNSPECIFIED"
	StorageStateAccepted      StorageState = "ACCEPTED"
	StorageStateReadyToReceive StorageState = "READY_TO_RECEIVE"
	StorageStateReceiving     StorageState = "RECEIVING"
	StorageStateStored        StorageState = "STORED"
	StorageStateFailed        StorageState = "FAILED"
)

type TransferState string

const (
	TransferStateUnspecified TransferState = "UNSPECIFIED"
	TransferStateOpen        TransferState = "OPEN"
	TransferStateReceiving   TransferState = "RECEIVING"
	TransferStateCompleted   TransferState = "COMPLETED"
	TransferStateRejected    TransferState = "REJECTED"
	TransferStateFailed      TransferState = "FAILED"
)

type DataSourceCategory string

const (
	DataSourceCategoryUnspecified DataSourceCategory = "UNSPECIFIED"
	DataSourceCategorySensingCSI  DataSourceCategory = "SENSING_CSI"
	DataSourceCategoryAI          DataSourceCategory = "AI"
)

type DataSourceScenario string

const (
	DataSourceScenarioUnspecified           DataSourceScenario = "UNSPECIFIED"
	DataSourceScenarioBreathingCSI          DataSourceScenario = "BREATHING_CSI"
	DataSourceScenarioVehicleCSI            DataSourceScenario = "VEHICLE_CSI"
	DataSourceScenarioPositioningCSI        DataSourceScenario = "POSITIONING_CSI"
	DataSourceScenarioGestureRecognitionCSI DataSourceScenario = "GESTURE_RECOGNITION_CSI"
	DataSourceScenarioLiquidRecognitionCSI  DataSourceScenario = "LIQUID_RECOGNITION_CSI"
	DataSourceScenarioModelDistribution     DataSourceScenario = "MODEL_DISTRIBUTION"
	DataSourceScenarioTokenTransmission     DataSourceScenario = "TOKEN_TRANSMISSION"
)

type Endpoint struct {
	Scheme string `json:"scheme"`
	Host   string `json:"host"`
	Port   uint32 `json:"port"`
	Path   string `json:"path"`
}

type KvPair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type DataSourceSpec struct {
	SourceID         string             `json:"sourceId"`
	SourceCategory   DataSourceCategory `json:"sourceCategory"`
	SourceScenario   DataSourceScenario `json:"sourceScenario"`
	SourceTypeDetail string             `json:"sourceTypeDetail"`
	IngressEndpoint  Endpoint           `json:"ingressEndpoint"`
	Encoding         string             `json:"encoding"`
	SubscribedTopics []string           `json:"subscribedTopics"`
	Attributes       []KvPair           `json:"attributes"`
}

type ProcessingStep struct {
	Name       string   `json:"name"`
	Version    string   `json:"version"`
	Parameters []KvPair `json:"parameters"`
}

type DeliveryBinding struct {
	DsfID              string       `json:"dsfId"`
	DsfControlEndpoint Endpoint     `json:"dsfControlEndpoint"`
	DsfDataEndpoint    Endpoint     `json:"dsfDataEndpoint"`
	TransportProtocol  ProtocolType `json:"transportProtocol"`
	PayloadProtocol    ProtocolType `json:"payloadProtocol"`
	ChannelID          string       `json:"channelId"`
	ProtocolParameters []KvPair     `json:"protocolParameters"`
}

type CallbackBinding struct {
	DsmfCallbackEndpoint Endpoint `json:"dsmfCallbackEndpoint"`
	CallbackRequestID    string   `json:"callbackRequestId"`
	CallbackTimeoutSeconds uint32 `json:"callbackTimeoutSeconds"`
	CallbackMetadata     []KvPair `json:"callbackMetadata"`
}

type SubmitProcessingTaskRequest struct {
	OrchestrationID   string           `json:"orchestrationId"`
	TaskID            string           `json:"taskId"`
	RequestID         string           `json:"requestId"`
	Source            DataSourceSpec   `json:"source"`
	ProcessingSteps   []ProcessingStep `json:"processingSteps"`
	OutputSchema      string           `json:"outputSchema"`
	Delivery          DeliveryBinding  `json:"delivery"`
	Callback          CallbackBinding  `json:"callback"`
	TaskTimeoutSeconds uint32          `json:"taskTimeoutSeconds"`
	Labels            []KvPair         `json:"labels"`
}

type SubmitProcessingTaskResponse struct {
	TaskID   string          `json:"taskId"`
	Accepted bool            `json:"accepted"`
	DpfID    string          `json:"dpfId"`
	State    ProcessingState `json:"state"`
	Reason   string          `json:"reason"`
}

type ProcessingMetrics struct {
	SourceRecords uint64 `json:"sourceRecords"`
	OutputRecords uint64 `json:"outputRecords"`
	SourceBytes   uint64 `json:"sourceBytes"`
	OutputBytes   uint64 `json:"outputBytes"`
}

type ProcessingStatusReport struct {
	OrchestrationID  string            `json:"orchestrationId"`
	TaskID           string            `json:"taskId"`
	DpfID            string            `json:"dpfId"`
	State            ProcessingState   `json:"state"`
	Detail           string            `json:"detail"`
	Metrics          ProcessingMetrics `json:"metrics"`
	TransferSessionID string           `json:"transferSessionId"`
	CallbackRequestID string           `json:"callbackRequestId"`
	ErrorCode        string            `json:"errorCode"`
	ErrorMessage     string            `json:"errorMessage"`
}

type ProcessingStatusAck struct {
	Accepted bool   `json:"accepted"`
	Message  string `json:"message"`
}

type StoragePolicy struct {
	BackendType       string   `json:"backendType"`
	BucketOrTable     string   `json:"bucketOrTable"`
	ObjectPrefix      string   `json:"objectPrefix"`
	RetentionDays     uint32   `json:"retentionDays"`
	OverwriteIfExists bool     `json:"overwriteIfExists"`
	Properties        []KvPair `json:"properties"`
}

type ReceiveContract struct {
	DpfID              string       `json:"dpfId"`
	TransportProtocol  ProtocolType `json:"transportProtocol"`
	PayloadProtocol    ProtocolType `json:"payloadProtocol"`
	ChannelID          string       `json:"channelId"`
	ReceiveEndpoint    Endpoint     `json:"receiveEndpoint"`
	ExpectedContentType string      `json:"expectedContentType"`
	ProtocolParameters []KvPair     `json:"protocolParameters"`
}

type SubmitStorageTaskRequest struct {
	OrchestrationID string         `json:"orchestrationId"`
	TaskID          string         `json:"taskId"`
	RequestID       string         `json:"requestId"`
	ReceiveContract ReceiveContract `json:"receiveContract"`
	StoragePolicy   StoragePolicy   `json:"storagePolicy"`
	ResultSchema    string          `json:"resultSchema"`
	Callback        CallbackBinding `json:"callback"`
	Labels          []KvPair        `json:"labels"`
}

type SubmitStorageTaskResponse struct {
	TaskID   string       `json:"taskId"`
	Accepted bool         `json:"accepted"`
	DsfID    string       `json:"dsfId"`
	State    StorageState `json:"state"`
	Reason   string       `json:"reason"`
}

type StorageResultLocation struct {
	ResultURI   string `json:"resultUri"`
	ResultID    string `json:"resultId"`
	Checksum    string `json:"checksum"`
	StoredBytes uint64 `json:"storedBytes"`
}

type StorageStatusReport struct {
	OrchestrationID  string                `json:"orchestrationId"`
	TaskID           string                `json:"taskId"`
	DsfID            string                `json:"dsfId"`
	State            StorageState          `json:"state"`
	Detail           string                `json:"detail"`
	ResultLocation   StorageResultLocation `json:"resultLocation"`
	CallbackRequestID string               `json:"callbackRequestId"`
	ErrorCode        string                `json:"errorCode"`
	ErrorMessage     string                `json:"errorMessage"`
}

type StorageStatusAck struct {
	Accepted bool   `json:"accepted"`
	Message  string `json:"message"`
}

type OpenTransferRequest struct {
	OrchestrationID   string       `json:"orchestrationId"`
	TaskID            string       `json:"taskId"`
	TransferSessionID string       `json:"transferSessionId"`
	DpfID             string       `json:"dpfId"`
	DsfID             string       `json:"dsfId"`
	TransportProtocol ProtocolType `json:"transportProtocol"`
	PayloadProtocol   ProtocolType `json:"payloadProtocol"`
	ChannelID         string       `json:"channelId"`
	ContentType       string       `json:"contentType"`
	PayloadSchema     string       `json:"payloadSchema"`
	ProtocolParameters []KvPair    `json:"protocolParameters"`
}

type OpenTransferResponse struct {
	Accepted          bool          `json:"accepted"`
	TransferSessionID string        `json:"transferSessionId"`
	State             TransferState `json:"state"`
	Reason            string        `json:"reason"`
}

type ResultChunk struct {
	OrchestrationID   string            `json:"orchestrationId"`
	TaskID            string            `json:"taskId"`
	TransferSessionID string            `json:"transferSessionId"`
	SequenceNo        uint64            `json:"sequenceNo"`
	Payload           []byte            `json:"payload"`
	Eof               bool              `json:"eof"`
	Checksum          string            `json:"checksum"`
	Metadata          map[string]string `json:"metadata"`
}

type PushResultResponse struct {
	TransferSessionID string        `json:"transferSessionId"`
	State             TransferState `json:"state"`
	ReceivedChunks    uint64        `json:"receivedChunks"`
	ReceivedBytes     uint64        `json:"receivedBytes"`
	Reason            string        `json:"reason"`
}

type CloseTransferRequest struct {
	OrchestrationID   string `json:"orchestrationId"`
	TaskID            string `json:"taskId"`
	TransferSessionID string `json:"transferSessionId"`
	TotalChunks       uint64 `json:"totalChunks"`
	TotalBytes        uint64 `json:"totalBytes"`
	FinalChecksum     string `json:"finalChecksum"`
}

type CloseTransferResponse struct {
	TransferSessionID string        `json:"transferSessionId"`
	State             TransferState `json:"state"`
	StorageTicket     string        `json:"storageTicket"`
	Message           string        `json:"message"`
}
