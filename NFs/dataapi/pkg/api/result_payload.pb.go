package api

import (
	"reflect"
	"sync"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoimpl"
	"google.golang.org/protobuf/types/descriptorpb"
)

type ResultParameter struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Key           string                 `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	Value         string                 `protobuf:"bytes,2,opt,name=value,proto3" json:"value,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ResultParameter) Reset() {
	*x = ResultParameter{}
	mi := &file_result_payload_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ResultParameter) String() string { return protoimpl.X.MessageStringOf(x) }
func (*ResultParameter) ProtoMessage()    {}

func (x *ResultParameter) ProtoReflect() protoreflect.Message {
	mi := &file_result_payload_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (x *ResultParameter) GetKey() string {
	if x != nil {
		return x.Key
	}
	return ""
}

func (x *ResultParameter) GetValue() string {
	if x != nil {
		return x.Value
	}
	return ""
}

type BreathingCsiResult struct {
	state             protoimpl.MessageState `protogen:"open.v1"`
	Mode              string                 `protobuf:"bytes,1,opt,name=mode,proto3" json:"mode,omitempty"`
	EstimatedRateBpm  float64                `protobuf:"fixed64,2,opt,name=estimated_rate_bpm,json=estimatedRateBpm,proto3" json:"estimated_rate_bpm,omitempty"`
	Confidence        float64                `protobuf:"fixed64,3,opt,name=confidence,proto3" json:"confidence,omitempty"`
	PipelineValidated bool                   `protobuf:"varint,4,opt,name=pipeline_validated,json=pipelineValidated,proto3" json:"pipeline_validated,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *BreathingCsiResult) Reset() {
	*x = BreathingCsiResult{}
	mi := &file_result_payload_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *BreathingCsiResult) String() string { return protoimpl.X.MessageStringOf(x) }
func (*BreathingCsiResult) ProtoMessage()    {}

func (x *BreathingCsiResult) ProtoReflect() protoreflect.Message {
	mi := &file_result_payload_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

type GestureRecognitionResult struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Mode          string                 `protobuf:"bytes,1,opt,name=mode,proto3" json:"mode,omitempty"`
	GestureLabel  string                 `protobuf:"bytes,2,opt,name=gesture_label,json=gestureLabel,proto3" json:"gesture_label,omitempty"`
	Confidence    float64                `protobuf:"fixed64,3,opt,name=confidence,proto3" json:"confidence,omitempty"`
	CsiTimeSteps  uint64                 `protobuf:"varint,4,opt,name=csi_time_steps,json=csiTimeSteps,proto3" json:"csi_time_steps,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GestureRecognitionResult) Reset() {
	*x = GestureRecognitionResult{}
	mi := &file_result_payload_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GestureRecognitionResult) String() string { return protoimpl.X.MessageStringOf(x) }
func (*GestureRecognitionResult) ProtoMessage()    {}

func (x *GestureRecognitionResult) ProtoReflect() protoreflect.Message {
	mi := &file_result_payload_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

type PositioningResult struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Mode          string                 `protobuf:"bytes,1,opt,name=mode,proto3" json:"mode,omitempty"`
	XMeters       float64                `protobuf:"fixed64,2,opt,name=x_meters,json=xMeters,proto3" json:"x_meters,omitempty"`
	YMeters       float64                `protobuf:"fixed64,3,opt,name=y_meters,json=yMeters,proto3" json:"y_meters,omitempty"`
	Confidence    float64                `protobuf:"fixed64,4,opt,name=confidence,proto3" json:"confidence,omitempty"`
	CsiTimeSteps  uint64                 `protobuf:"varint,5,opt,name=csi_time_steps,json=csiTimeSteps,proto3" json:"csi_time_steps,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *PositioningResult) Reset() {
	*x = PositioningResult{}
	mi := &file_result_payload_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *PositioningResult) String() string { return protoimpl.X.MessageStringOf(x) }
func (*PositioningResult) ProtoMessage()    {}

func (x *PositioningResult) ProtoReflect() protoreflect.Message {
	mi := &file_result_payload_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

type VehicleResult struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Mode          string                 `protobuf:"bytes,1,opt,name=mode,proto3" json:"mode,omitempty"`
	MobilityState string                 `protobuf:"bytes,2,opt,name=mobility_state,json=mobilityState,proto3" json:"mobility_state,omitempty"`
	SpeedMps      float64                `protobuf:"fixed64,3,opt,name=speed_mps,json=speedMps,proto3" json:"speed_mps,omitempty"`
	Confidence    float64                `protobuf:"fixed64,4,opt,name=confidence,proto3" json:"confidence,omitempty"`
	CsiTimeSteps  uint64                 `protobuf:"varint,5,opt,name=csi_time_steps,json=csiTimeSteps,proto3" json:"csi_time_steps,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *VehicleResult) Reset() {
	*x = VehicleResult{}
	mi := &file_result_payload_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *VehicleResult) String() string { return protoimpl.X.MessageStringOf(x) }
func (*VehicleResult) ProtoMessage()    {}

func (x *VehicleResult) ProtoReflect() protoreflect.Message {
	mi := &file_result_payload_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

type ProcessingResult struct {
	state              protoimpl.MessageState    `protogen:"open.v1"`
	TaskId             string                    `protobuf:"bytes,1,opt,name=task_id,json=taskId,proto3" json:"task_id,omitempty"`
	RequestId          string                    `protobuf:"bytes,2,opt,name=request_id,json=requestId,proto3" json:"request_id,omitempty"`
	SourceId           string                    `protobuf:"bytes,3,opt,name=source_id,json=sourceId,proto3" json:"source_id,omitempty"`
	SourceCategory     string                    `protobuf:"bytes,4,opt,name=source_category,json=sourceCategory,proto3" json:"source_category,omitempty"`
	SourceScenario     string                    `protobuf:"bytes,5,opt,name=source_scenario,json=sourceScenario,proto3" json:"source_scenario,omitempty"`
	OutputSchema       string                    `protobuf:"bytes,6,opt,name=output_schema,json=outputSchema,proto3" json:"output_schema,omitempty"`
	StepsApplied       []string                  `protobuf:"bytes,7,rep,name=steps_applied,json=stepsApplied,proto3" json:"steps_applied,omitempty"`
	SourceBytes        uint64                    `protobuf:"varint,8,opt,name=source_bytes,json=sourceBytes,proto3" json:"source_bytes,omitempty"`
	ContentPreview     string                    `protobuf:"bytes,9,opt,name=content_preview,json=contentPreview,proto3" json:"content_preview,omitempty"`
	Parameters         []*ResultParameter        `protobuf:"bytes,10,rep,name=parameters,proto3" json:"parameters,omitempty"`
	ProcessedBy        string                    `protobuf:"bytes,11,opt,name=processed_by,json=processedBy,proto3" json:"processed_by,omitempty"`
	ProcessedAt        string                    `protobuf:"bytes,12,opt,name=processed_at,json=processedAt,proto3" json:"processed_at,omitempty"`
	BreathingCsi       *BreathingCsiResult       `protobuf:"bytes,13,opt,name=breathing_csi,json=breathingCsi,proto3" json:"breathing_csi,omitempty"`
	GestureRecognition *GestureRecognitionResult `protobuf:"bytes,14,opt,name=gesture_recognition,json=gestureRecognition,proto3" json:"gesture_recognition,omitempty"`
	Positioning        *PositioningResult        `protobuf:"bytes,15,opt,name=positioning,proto3" json:"positioning,omitempty"`
	Vehicle            *VehicleResult            `protobuf:"bytes,16,opt,name=vehicle,proto3" json:"vehicle,omitempty"`
	unknownFields      protoimpl.UnknownFields
	sizeCache          protoimpl.SizeCache
}

func (x *ProcessingResult) Reset() {
	*x = ProcessingResult{}
	mi := &file_result_payload_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ProcessingResult) String() string { return protoimpl.X.MessageStringOf(x) }
func (*ProcessingResult) ProtoMessage()    {}

func (x *ProcessingResult) ProtoReflect() protoreflect.Message {
	mi := &file_result_payload_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

var File_result_payload_proto protoreflect.FileDescriptor

var file_result_payload_proto_rawDescOnce sync.Once
var file_result_payload_proto_rawDescData []byte

var file_result_payload_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_result_payload_proto_goTypes = []any{
	(*ResultParameter)(nil),
	(*BreathingCsiResult)(nil),
	(*GestureRecognitionResult)(nil),
	(*PositioningResult)(nil),
	(*VehicleResult)(nil),
	(*ProcessingResult)(nil),
}

func file_result_payload_proto_rawDescGZIP() []byte {
	file_result_payload_proto_rawDescOnce.Do(func() {
		file_result_payload_proto_rawDescData = protoimpl.X.CompressGZIP(file_result_payload_proto_rawDescData)
	})
	return file_result_payload_proto_rawDescData
}

func init() { file_result_payload_proto_init() }

func file_result_payload_proto_init() {
	if File_result_payload_proto != nil {
		return
	}
	file_result_payload_proto_rawDescData = buildResultPayloadRawDescriptor()
	if !protoimpl.UnsafeEnabled {
		file_result_payload_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*ResultParameter); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_result_payload_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*BreathingCsiResult); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_result_payload_proto_msgTypes[2].Exporter = func(v any, i int) any {
			switch v := v.(*GestureRecognitionResult); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_result_payload_proto_msgTypes[3].Exporter = func(v any, i int) any {
			switch v := v.(*PositioningResult); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_result_payload_proto_msgTypes[4].Exporter = func(v any, i int) any {
			switch v := v.(*VehicleResult); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_result_payload_proto_msgTypes[5].Exporter = func(v any, i int) any {
			switch v := v.(*ProcessingResult); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_result_payload_proto_rawDescData,
			NumEnums:      0,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes: file_result_payload_proto_goTypes,
		DependencyIndexes: []int32{
			0, // 0: ProcessingResult.parameters:type_name -> ResultParameter
			1, // 1: ProcessingResult.breathing_csi:type_name -> BreathingCsiResult
			2, // 2: ProcessingResult.gesture_recognition:type_name -> GestureRecognitionResult
			3, // 3: ProcessingResult.positioning:type_name -> PositioningResult
			4, // 4: ProcessingResult.vehicle:type_name -> VehicleResult
			0, 1, 2, 3, 4,
		},
		MessageInfos: file_result_payload_proto_msgTypes,
	}.Build()
	File_result_payload_proto = out.File
	file_result_payload_proto_rawDescData = nil
	file_result_payload_proto_goTypes = nil
}

func buildResultPayloadRawDescriptor() []byte {
	labelOptional := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL
	labelRepeated := descriptorpb.FieldDescriptorProto_LABEL_REPEATED
	typeString := descriptorpb.FieldDescriptorProto_TYPE_STRING
	typeDouble := descriptorpb.FieldDescriptorProto_TYPE_DOUBLE
	typeBool := descriptorpb.FieldDescriptorProto_TYPE_BOOL
	typeUint64 := descriptorpb.FieldDescriptorProto_TYPE_UINT64
	typeMessage := descriptorpb.FieldDescriptorProto_TYPE_MESSAGE

	field := func(name string, number int32, label descriptorpb.FieldDescriptorProto_Label, typ descriptorpb.FieldDescriptorProto_Type, typeName string, jsonName string) *descriptorpb.FieldDescriptorProto {
		f := &descriptorpb.FieldDescriptorProto{
			Name:   proto.String(name),
			Number: proto.Int32(number),
			Label:  &label,
			Type:   &typ,
		}
		if typeName != "" {
			f.TypeName = proto.String(typeName)
		}
		if jsonName != "" {
			f.JsonName = proto.String(jsonName)
		}
		return f
	}
	message := func(name string, fields ...*descriptorpb.FieldDescriptorProto) *descriptorpb.DescriptorProto {
		return &descriptorpb.DescriptorProto{Name: proto.String(name), Field: fields}
	}

	fd := &descriptorpb.FileDescriptorProto{
		Syntax:  proto.String("proto3"),
		Name:    proto.String("result_payload.proto"),
		Package: proto.String("net.framework.result_payload.v1"),
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String("github.com/free5gc/dataapi/pkg/api"),
		},
		MessageType: []*descriptorpb.DescriptorProto{
			message("ResultParameter",
				field("key", 1, labelOptional, typeString, "", "key"),
				field("value", 2, labelOptional, typeString, "", "value"),
			),
			message("BreathingCsiResult",
				field("mode", 1, labelOptional, typeString, "", "mode"),
				field("estimated_rate_bpm", 2, labelOptional, typeDouble, "", "estimatedRateBpm"),
				field("confidence", 3, labelOptional, typeDouble, "", "confidence"),
				field("pipeline_validated", 4, labelOptional, typeBool, "", "pipelineValidated"),
			),
			message("GestureRecognitionResult",
				field("mode", 1, labelOptional, typeString, "", "mode"),
				field("gesture_label", 2, labelOptional, typeString, "", "gestureLabel"),
				field("confidence", 3, labelOptional, typeDouble, "", "confidence"),
				field("csi_time_steps", 4, labelOptional, typeUint64, "", "csiTimeSteps"),
			),
			message("PositioningResult",
				field("mode", 1, labelOptional, typeString, "", "mode"),
				field("x_meters", 2, labelOptional, typeDouble, "", "xMeters"),
				field("y_meters", 3, labelOptional, typeDouble, "", "yMeters"),
				field("confidence", 4, labelOptional, typeDouble, "", "confidence"),
				field("csi_time_steps", 5, labelOptional, typeUint64, "", "csiTimeSteps"),
			),
			message("VehicleResult",
				field("mode", 1, labelOptional, typeString, "", "mode"),
				field("mobility_state", 2, labelOptional, typeString, "", "mobilityState"),
				field("speed_mps", 3, labelOptional, typeDouble, "", "speedMps"),
				field("confidence", 4, labelOptional, typeDouble, "", "confidence"),
				field("csi_time_steps", 5, labelOptional, typeUint64, "", "csiTimeSteps"),
			),
			message("ProcessingResult",
				field("task_id", 1, labelOptional, typeString, "", "taskId"),
				field("request_id", 2, labelOptional, typeString, "", "requestId"),
				field("source_id", 3, labelOptional, typeString, "", "sourceId"),
				field("source_category", 4, labelOptional, typeString, "", "sourceCategory"),
				field("source_scenario", 5, labelOptional, typeString, "", "sourceScenario"),
				field("output_schema", 6, labelOptional, typeString, "", "outputSchema"),
				field("steps_applied", 7, labelRepeated, typeString, "", "stepsApplied"),
				field("source_bytes", 8, labelOptional, typeUint64, "", "sourceBytes"),
				field("content_preview", 9, labelOptional, typeString, "", "contentPreview"),
				field("parameters", 10, labelRepeated, typeMessage, ".net.framework.result_payload.v1.ResultParameter", "parameters"),
				field("processed_by", 11, labelOptional, typeString, "", "processedBy"),
				field("processed_at", 12, labelOptional, typeString, "", "processedAt"),
				field("breathing_csi", 13, labelOptional, typeMessage, ".net.framework.result_payload.v1.BreathingCsiResult", "breathingCsi"),
				field("gesture_recognition", 14, labelOptional, typeMessage, ".net.framework.result_payload.v1.GestureRecognitionResult", "gestureRecognition"),
				field("positioning", 15, labelOptional, typeMessage, ".net.framework.result_payload.v1.PositioningResult", "positioning"),
				field("vehicle", 16, labelOptional, typeMessage, ".net.framework.result_payload.v1.VehicleResult", "vehicle"),
			),
		},
	}

	raw, err := proto.Marshal(fd)
	if err != nil {
		panic(err)
	}
	return raw
}
