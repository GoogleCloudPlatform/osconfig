// Code generated by protoc-gen-go. DO NOT EDIT.
// source: google/cloud/osconfig/agentendpoint/v1alpha1/agentendpoint.proto

package agentendpoint

import (
	context "context"
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	_ "google.golang.org/genproto/googleapis/api/annotations"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

// A request message to receive task notifications.
type ReceiveTaskNotificationRequest struct {
	// Required. This is the GCE instance identity token described in
	// https://cloud.google.com/compute/docs/instances/verifying-instance-identity
	// where the audience is 'osconfig.googleapis.com' and the format is 'full'.
	InstanceIdToken string `protobuf:"bytes,1,opt,name=instance_id_token,json=instanceIdToken,proto3" json:"instance_id_token,omitempty"`
	// Required. The version of the agent making the request.
	AgentVersion         string   `protobuf:"bytes,2,opt,name=agent_version,json=agentVersion,proto3" json:"agent_version,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ReceiveTaskNotificationRequest) Reset()         { *m = ReceiveTaskNotificationRequest{} }
func (m *ReceiveTaskNotificationRequest) String() string { return proto.CompactTextString(m) }
func (*ReceiveTaskNotificationRequest) ProtoMessage()    {}
func (*ReceiveTaskNotificationRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_a8eb4534d0795eb1, []int{0}
}

func (m *ReceiveTaskNotificationRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ReceiveTaskNotificationRequest.Unmarshal(m, b)
}
func (m *ReceiveTaskNotificationRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ReceiveTaskNotificationRequest.Marshal(b, m, deterministic)
}
func (m *ReceiveTaskNotificationRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ReceiveTaskNotificationRequest.Merge(m, src)
}
func (m *ReceiveTaskNotificationRequest) XXX_Size() int {
	return xxx_messageInfo_ReceiveTaskNotificationRequest.Size(m)
}
func (m *ReceiveTaskNotificationRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_ReceiveTaskNotificationRequest.DiscardUnknown(m)
}

var xxx_messageInfo_ReceiveTaskNotificationRequest proto.InternalMessageInfo

func (m *ReceiveTaskNotificationRequest) GetInstanceIdToken() string {
	if m != nil {
		return m.InstanceIdToken
	}
	return ""
}

func (m *ReceiveTaskNotificationRequest) GetAgentVersion() string {
	if m != nil {
		return m.AgentVersion
	}
	return ""
}

// The streaming rpc message that will notify the agent when it has a task
// it needs to perform on the instance.
type ReceiveTaskNotificationResponse struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ReceiveTaskNotificationResponse) Reset()         { *m = ReceiveTaskNotificationResponse{} }
func (m *ReceiveTaskNotificationResponse) String() string { return proto.CompactTextString(m) }
func (*ReceiveTaskNotificationResponse) ProtoMessage()    {}
func (*ReceiveTaskNotificationResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_a8eb4534d0795eb1, []int{1}
}

func (m *ReceiveTaskNotificationResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ReceiveTaskNotificationResponse.Unmarshal(m, b)
}
func (m *ReceiveTaskNotificationResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ReceiveTaskNotificationResponse.Marshal(b, m, deterministic)
}
func (m *ReceiveTaskNotificationResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ReceiveTaskNotificationResponse.Merge(m, src)
}
func (m *ReceiveTaskNotificationResponse) XXX_Size() int {
	return xxx_messageInfo_ReceiveTaskNotificationResponse.Size(m)
}
func (m *ReceiveTaskNotificationResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_ReceiveTaskNotificationResponse.DiscardUnknown(m)
}

var xxx_messageInfo_ReceiveTaskNotificationResponse proto.InternalMessageInfo

// A request message for signaling the start of a task execution.
type ReportTaskStartRequest struct {
	// Required. This is the GCE instance identity token described in
	// https://cloud.google.com/compute/docs/instances/verifying-instance-identity
	// where the audience is 'osconfig.googleapis.com' and the format is 'full'.
	InstanceIdToken      string   `protobuf:"bytes,1,opt,name=instance_id_token,json=instanceIdToken,proto3" json:"instance_id_token,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ReportTaskStartRequest) Reset()         { *m = ReportTaskStartRequest{} }
func (m *ReportTaskStartRequest) String() string { return proto.CompactTextString(m) }
func (*ReportTaskStartRequest) ProtoMessage()    {}
func (*ReportTaskStartRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_a8eb4534d0795eb1, []int{2}
}

func (m *ReportTaskStartRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ReportTaskStartRequest.Unmarshal(m, b)
}
func (m *ReportTaskStartRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ReportTaskStartRequest.Marshal(b, m, deterministic)
}
func (m *ReportTaskStartRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ReportTaskStartRequest.Merge(m, src)
}
func (m *ReportTaskStartRequest) XXX_Size() int {
	return xxx_messageInfo_ReportTaskStartRequest.Size(m)
}
func (m *ReportTaskStartRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_ReportTaskStartRequest.DiscardUnknown(m)
}

var xxx_messageInfo_ReportTaskStartRequest proto.InternalMessageInfo

func (m *ReportTaskStartRequest) GetInstanceIdToken() string {
	if m != nil {
		return m.InstanceIdToken
	}
	return ""
}

// A response message that contains the details of the task to work on.
type ReportTaskStartResponse struct {
	// The details of the task that should be worked on.  Can be empty if there
	// is no new task to work on.
	Task                 *Task    `protobuf:"bytes,1,opt,name=task,proto3" json:"task,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ReportTaskStartResponse) Reset()         { *m = ReportTaskStartResponse{} }
func (m *ReportTaskStartResponse) String() string { return proto.CompactTextString(m) }
func (*ReportTaskStartResponse) ProtoMessage()    {}
func (*ReportTaskStartResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_a8eb4534d0795eb1, []int{3}
}

func (m *ReportTaskStartResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ReportTaskStartResponse.Unmarshal(m, b)
}
func (m *ReportTaskStartResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ReportTaskStartResponse.Marshal(b, m, deterministic)
}
func (m *ReportTaskStartResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ReportTaskStartResponse.Merge(m, src)
}
func (m *ReportTaskStartResponse) XXX_Size() int {
	return xxx_messageInfo_ReportTaskStartResponse.Size(m)
}
func (m *ReportTaskStartResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_ReportTaskStartResponse.DiscardUnknown(m)
}

var xxx_messageInfo_ReportTaskStartResponse proto.InternalMessageInfo

func (m *ReportTaskStartResponse) GetTask() *Task {
	if m != nil {
		return m.Task
	}
	return nil
}

// A request message for reporting the progress of current task.
type ReportTaskProgressRequest struct {
	// Required. This is the GCE instance identity token described in
	// https://cloud.google.com/compute/docs/instances/verifying-instance-identity
	// where the audience is 'osconfig.googleapis.com' and the format is 'full'.
	InstanceIdToken string `protobuf:"bytes,1,opt,name=instance_id_token,json=instanceIdToken,proto3" json:"instance_id_token,omitempty"`
	// Required. Unique identifier of the task this applies to.
	TaskId string `protobuf:"bytes,2,opt,name=task_id,json=taskId,proto3" json:"task_id,omitempty"`
	// Required. The type of task to report progress on.
	//
	// Progress must include the appropriate message based on this enum as
	// specified below:
	// APPLY_PATCHES = ApplyPatchesTaskProgress
	// EXEC_STEP = Progress not supported for this type.
	TaskType TaskType `protobuf:"varint,3,opt,name=task_type,json=taskType,proto3,enum=google.cloud.osconfig.agentendpoint.v1alpha1.TaskType" json:"task_type,omitempty"`
	// Intermediate progress of the current task.
	//
	// Types that are valid to be assigned to Progress:
	//	*ReportTaskProgressRequest_ApplyPatchesTaskProgress
	//	*ReportTaskProgressRequest_ExecStepTaskProgress
	Progress             isReportTaskProgressRequest_Progress `protobuf_oneof:"progress"`
	XXX_NoUnkeyedLiteral struct{}                             `json:"-"`
	XXX_unrecognized     []byte                               `json:"-"`
	XXX_sizecache        int32                                `json:"-"`
}

func (m *ReportTaskProgressRequest) Reset()         { *m = ReportTaskProgressRequest{} }
func (m *ReportTaskProgressRequest) String() string { return proto.CompactTextString(m) }
func (*ReportTaskProgressRequest) ProtoMessage()    {}
func (*ReportTaskProgressRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_a8eb4534d0795eb1, []int{4}
}

func (m *ReportTaskProgressRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ReportTaskProgressRequest.Unmarshal(m, b)
}
func (m *ReportTaskProgressRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ReportTaskProgressRequest.Marshal(b, m, deterministic)
}
func (m *ReportTaskProgressRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ReportTaskProgressRequest.Merge(m, src)
}
func (m *ReportTaskProgressRequest) XXX_Size() int {
	return xxx_messageInfo_ReportTaskProgressRequest.Size(m)
}
func (m *ReportTaskProgressRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_ReportTaskProgressRequest.DiscardUnknown(m)
}

var xxx_messageInfo_ReportTaskProgressRequest proto.InternalMessageInfo

func (m *ReportTaskProgressRequest) GetInstanceIdToken() string {
	if m != nil {
		return m.InstanceIdToken
	}
	return ""
}

func (m *ReportTaskProgressRequest) GetTaskId() string {
	if m != nil {
		return m.TaskId
	}
	return ""
}

func (m *ReportTaskProgressRequest) GetTaskType() TaskType {
	if m != nil {
		return m.TaskType
	}
	return TaskType_TASK_TYPE_UNSPECIFIED
}

type isReportTaskProgressRequest_Progress interface {
	isReportTaskProgressRequest_Progress()
}

type ReportTaskProgressRequest_ApplyPatchesTaskProgress struct {
	ApplyPatchesTaskProgress *ApplyPatchesTaskProgress `protobuf:"bytes,4,opt,name=apply_patches_task_progress,json=applyPatchesTaskProgress,proto3,oneof"`
}

type ReportTaskProgressRequest_ExecStepTaskProgress struct {
	ExecStepTaskProgress *ExecStepTaskProgress `protobuf:"bytes,5,opt,name=exec_step_task_progress,json=execStepTaskProgress,proto3,oneof"`
}

func (*ReportTaskProgressRequest_ApplyPatchesTaskProgress) isReportTaskProgressRequest_Progress() {}

func (*ReportTaskProgressRequest_ExecStepTaskProgress) isReportTaskProgressRequest_Progress() {}

func (m *ReportTaskProgressRequest) GetProgress() isReportTaskProgressRequest_Progress {
	if m != nil {
		return m.Progress
	}
	return nil
}

func (m *ReportTaskProgressRequest) GetApplyPatchesTaskProgress() *ApplyPatchesTaskProgress {
	if x, ok := m.GetProgress().(*ReportTaskProgressRequest_ApplyPatchesTaskProgress); ok {
		return x.ApplyPatchesTaskProgress
	}
	return nil
}

func (m *ReportTaskProgressRequest) GetExecStepTaskProgress() *ExecStepTaskProgress {
	if x, ok := m.GetProgress().(*ReportTaskProgressRequest_ExecStepTaskProgress); ok {
		return x.ExecStepTaskProgress
	}
	return nil
}

// XXX_OneofWrappers is for the internal use of the proto package.
func (*ReportTaskProgressRequest) XXX_OneofWrappers() []interface{} {
	return []interface{}{
		(*ReportTaskProgressRequest_ApplyPatchesTaskProgress)(nil),
		(*ReportTaskProgressRequest_ExecStepTaskProgress)(nil),
	}
}

// The response message after the agent reported the current task progress.
type ReportTaskProgressResponse struct {
	// Instructs agent to continue or not.
	TaskDirective        TaskDirective `protobuf:"varint,1,opt,name=task_directive,json=taskDirective,proto3,enum=google.cloud.osconfig.agentendpoint.v1alpha1.TaskDirective" json:"task_directive,omitempty"`
	XXX_NoUnkeyedLiteral struct{}      `json:"-"`
	XXX_unrecognized     []byte        `json:"-"`
	XXX_sizecache        int32         `json:"-"`
}

func (m *ReportTaskProgressResponse) Reset()         { *m = ReportTaskProgressResponse{} }
func (m *ReportTaskProgressResponse) String() string { return proto.CompactTextString(m) }
func (*ReportTaskProgressResponse) ProtoMessage()    {}
func (*ReportTaskProgressResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_a8eb4534d0795eb1, []int{5}
}

func (m *ReportTaskProgressResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ReportTaskProgressResponse.Unmarshal(m, b)
}
func (m *ReportTaskProgressResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ReportTaskProgressResponse.Marshal(b, m, deterministic)
}
func (m *ReportTaskProgressResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ReportTaskProgressResponse.Merge(m, src)
}
func (m *ReportTaskProgressResponse) XXX_Size() int {
	return xxx_messageInfo_ReportTaskProgressResponse.Size(m)
}
func (m *ReportTaskProgressResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_ReportTaskProgressResponse.DiscardUnknown(m)
}

var xxx_messageInfo_ReportTaskProgressResponse proto.InternalMessageInfo

func (m *ReportTaskProgressResponse) GetTaskDirective() TaskDirective {
	if m != nil {
		return m.TaskDirective
	}
	return TaskDirective_TASK_DIRECTIVE_UNSPECIFIED
}

// A request message for signaling the completion of a task execution.
type ReportTaskCompleteRequest struct {
	// Required. This is the GCE instance identity token described in
	// https://cloud.google.com/compute/docs/instances/verifying-instance-identity
	// where the audience is 'osconfig.googleapis.com' and the format is 'full'.
	InstanceIdToken string `protobuf:"bytes,1,opt,name=instance_id_token,json=instanceIdToken,proto3" json:"instance_id_token,omitempty"`
	// Required. Unique identifier of the task this applies to.
	TaskId string `protobuf:"bytes,2,opt,name=task_id,json=taskId,proto3" json:"task_id,omitempty"`
	// Required. The type of task to report completed.
	//
	// Output must include the appropriate message based on this enum as
	// specified below:
	// APPLY_PATCHES = ApplyPatchesTaskOutput
	// EXEC_STEP = ExecStepTaskOutput
	TaskType TaskType `protobuf:"varint,3,opt,name=task_type,json=taskType,proto3,enum=google.cloud.osconfig.agentendpoint.v1alpha1.TaskType" json:"task_type,omitempty"`
	// Descriptive error message if the task execution ended in error.
	ErrorMessage string `protobuf:"bytes,4,opt,name=error_message,json=errorMessage,proto3" json:"error_message,omitempty"`
	// Final output details of the current task.
	//
	// Types that are valid to be assigned to Output:
	//	*ReportTaskCompleteRequest_ApplyPatchesTaskOutput
	//	*ReportTaskCompleteRequest_ExecStepTaskOutput
	Output               isReportTaskCompleteRequest_Output `protobuf_oneof:"output"`
	XXX_NoUnkeyedLiteral struct{}                           `json:"-"`
	XXX_unrecognized     []byte                             `json:"-"`
	XXX_sizecache        int32                              `json:"-"`
}

func (m *ReportTaskCompleteRequest) Reset()         { *m = ReportTaskCompleteRequest{} }
func (m *ReportTaskCompleteRequest) String() string { return proto.CompactTextString(m) }
func (*ReportTaskCompleteRequest) ProtoMessage()    {}
func (*ReportTaskCompleteRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_a8eb4534d0795eb1, []int{6}
}

func (m *ReportTaskCompleteRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ReportTaskCompleteRequest.Unmarshal(m, b)
}
func (m *ReportTaskCompleteRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ReportTaskCompleteRequest.Marshal(b, m, deterministic)
}
func (m *ReportTaskCompleteRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ReportTaskCompleteRequest.Merge(m, src)
}
func (m *ReportTaskCompleteRequest) XXX_Size() int {
	return xxx_messageInfo_ReportTaskCompleteRequest.Size(m)
}
func (m *ReportTaskCompleteRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_ReportTaskCompleteRequest.DiscardUnknown(m)
}

var xxx_messageInfo_ReportTaskCompleteRequest proto.InternalMessageInfo

func (m *ReportTaskCompleteRequest) GetInstanceIdToken() string {
	if m != nil {
		return m.InstanceIdToken
	}
	return ""
}

func (m *ReportTaskCompleteRequest) GetTaskId() string {
	if m != nil {
		return m.TaskId
	}
	return ""
}

func (m *ReportTaskCompleteRequest) GetTaskType() TaskType {
	if m != nil {
		return m.TaskType
	}
	return TaskType_TASK_TYPE_UNSPECIFIED
}

func (m *ReportTaskCompleteRequest) GetErrorMessage() string {
	if m != nil {
		return m.ErrorMessage
	}
	return ""
}

type isReportTaskCompleteRequest_Output interface {
	isReportTaskCompleteRequest_Output()
}

type ReportTaskCompleteRequest_ApplyPatchesTaskOutput struct {
	ApplyPatchesTaskOutput *ApplyPatchesTaskOutput `protobuf:"bytes,5,opt,name=apply_patches_task_output,json=applyPatchesTaskOutput,proto3,oneof"`
}

type ReportTaskCompleteRequest_ExecStepTaskOutput struct {
	ExecStepTaskOutput *ExecStepTaskOutput `protobuf:"bytes,6,opt,name=exec_step_task_output,json=execStepTaskOutput,proto3,oneof"`
}

func (*ReportTaskCompleteRequest_ApplyPatchesTaskOutput) isReportTaskCompleteRequest_Output() {}

func (*ReportTaskCompleteRequest_ExecStepTaskOutput) isReportTaskCompleteRequest_Output() {}

func (m *ReportTaskCompleteRequest) GetOutput() isReportTaskCompleteRequest_Output {
	if m != nil {
		return m.Output
	}
	return nil
}

func (m *ReportTaskCompleteRequest) GetApplyPatchesTaskOutput() *ApplyPatchesTaskOutput {
	if x, ok := m.GetOutput().(*ReportTaskCompleteRequest_ApplyPatchesTaskOutput); ok {
		return x.ApplyPatchesTaskOutput
	}
	return nil
}

func (m *ReportTaskCompleteRequest) GetExecStepTaskOutput() *ExecStepTaskOutput {
	if x, ok := m.GetOutput().(*ReportTaskCompleteRequest_ExecStepTaskOutput); ok {
		return x.ExecStepTaskOutput
	}
	return nil
}

// XXX_OneofWrappers is for the internal use of the proto package.
func (*ReportTaskCompleteRequest) XXX_OneofWrappers() []interface{} {
	return []interface{}{
		(*ReportTaskCompleteRequest_ApplyPatchesTaskOutput)(nil),
		(*ReportTaskCompleteRequest_ExecStepTaskOutput)(nil),
	}
}

// The response message after the agent signaled the current task complete.
type ReportTaskCompleteResponse struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ReportTaskCompleteResponse) Reset()         { *m = ReportTaskCompleteResponse{} }
func (m *ReportTaskCompleteResponse) String() string { return proto.CompactTextString(m) }
func (*ReportTaskCompleteResponse) ProtoMessage()    {}
func (*ReportTaskCompleteResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_a8eb4534d0795eb1, []int{7}
}

func (m *ReportTaskCompleteResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ReportTaskCompleteResponse.Unmarshal(m, b)
}
func (m *ReportTaskCompleteResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ReportTaskCompleteResponse.Marshal(b, m, deterministic)
}
func (m *ReportTaskCompleteResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ReportTaskCompleteResponse.Merge(m, src)
}
func (m *ReportTaskCompleteResponse) XXX_Size() int {
	return xxx_messageInfo_ReportTaskCompleteResponse.Size(m)
}
func (m *ReportTaskCompleteResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_ReportTaskCompleteResponse.DiscardUnknown(m)
}

var xxx_messageInfo_ReportTaskCompleteResponse proto.InternalMessageInfo

func init() {
	proto.RegisterType((*ReceiveTaskNotificationRequest)(nil), "google.cloud.osconfig.agentendpoint.v1alpha1.ReceiveTaskNotificationRequest")
	proto.RegisterType((*ReceiveTaskNotificationResponse)(nil), "google.cloud.osconfig.agentendpoint.v1alpha1.ReceiveTaskNotificationResponse")
	proto.RegisterType((*ReportTaskStartRequest)(nil), "google.cloud.osconfig.agentendpoint.v1alpha1.ReportTaskStartRequest")
	proto.RegisterType((*ReportTaskStartResponse)(nil), "google.cloud.osconfig.agentendpoint.v1alpha1.ReportTaskStartResponse")
	proto.RegisterType((*ReportTaskProgressRequest)(nil), "google.cloud.osconfig.agentendpoint.v1alpha1.ReportTaskProgressRequest")
	proto.RegisterType((*ReportTaskProgressResponse)(nil), "google.cloud.osconfig.agentendpoint.v1alpha1.ReportTaskProgressResponse")
	proto.RegisterType((*ReportTaskCompleteRequest)(nil), "google.cloud.osconfig.agentendpoint.v1alpha1.ReportTaskCompleteRequest")
	proto.RegisterType((*ReportTaskCompleteResponse)(nil), "google.cloud.osconfig.agentendpoint.v1alpha1.ReportTaskCompleteResponse")
}

func init() {
	proto.RegisterFile("google/cloud/osconfig/agentendpoint/v1alpha1/agentendpoint.proto", fileDescriptor_a8eb4534d0795eb1)
}

var fileDescriptor_a8eb4534d0795eb1 = []byte{
	// 780 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xd4, 0x96, 0x4d, 0x4f, 0xdb, 0x48,
	0x18, 0xc7, 0x63, 0x60, 0x03, 0xcc, 0xf2, 0xa2, 0x1d, 0xb1, 0x24, 0x78, 0xd1, 0xc2, 0x7a, 0x2f,
	0x1c, 0x56, 0x36, 0x64, 0xa5, 0xd5, 0x4a, 0x5c, 0x48, 0x96, 0xf0, 0x22, 0xc1, 0x36, 0x35, 0xa8,
	0x6a, 0x7b, 0xb1, 0x06, 0xfb, 0x89, 0x19, 0xc5, 0xf1, 0x4c, 0x3d, 0x93, 0x08, 0xc4, 0xa5, 0x3d,
	0xf5, 0x6b, 0x54, 0xea, 0xa9, 0xf7, 0x9e, 0xfa, 0x05, 0x2a, 0xf5, 0x0b, 0xf4, 0xda, 0x4f, 0xd0,
	0xcf, 0x50, 0x79, 0x6c, 0x93, 0x17, 0x02, 0xaa, 0x03, 0x97, 0x1e, 0xf3, 0x3c, 0xf6, 0xef, 0xff,
	0xf7, 0xcc, 0x3f, 0xcf, 0x0c, 0xda, 0xf1, 0x19, 0xf3, 0x03, 0xb0, 0xdc, 0x80, 0x75, 0x3c, 0x8b,
	0x09, 0x97, 0x85, 0x4d, 0xea, 0x5b, 0xc4, 0x87, 0x50, 0x42, 0xe8, 0x71, 0x46, 0x43, 0x69, 0x75,
	0xb7, 0x48, 0xc0, 0xcf, 0xc9, 0xd6, 0x60, 0xd9, 0xe4, 0x11, 0x93, 0x0c, 0xff, 0x95, 0x10, 0x4c,
	0x45, 0x30, 0x33, 0x82, 0x39, 0xf8, 0x68, 0x46, 0xd0, 0xd7, 0x52, 0x3d, 0xc2, 0xa9, 0xd5, 0xa4,
	0x10, 0x78, 0xce, 0x19, 0x9c, 0x93, 0x2e, 0x65, 0x51, 0x82, 0xd3, 0xab, 0xb9, 0x0c, 0xf9, 0x1d,
	0x10, 0xd2, 0xe1, 0x2c, 0xa0, 0x2e, 0x05, 0x91, 0x22, 0xfe, 0xcd, 0x85, 0x90, 0x44, 0xb4, 0xb2,
	0x37, 0x4b, 0x7d, 0xee, 0xdc, 0x80, 0x42, 0xf6, 0x91, 0xc6, 0x15, 0xfa, 0xdd, 0x06, 0x17, 0x68,
	0x17, 0x4e, 0x89, 0x68, 0xfd, 0xcf, 0x24, 0x6d, 0x52, 0x97, 0x48, 0xca, 0x42, 0x1b, 0x5e, 0xc4,
	0x1e, 0xb0, 0x85, 0x7e, 0xa1, 0xa1, 0x90, 0x24, 0x74, 0xc1, 0xa1, 0x9e, 0x23, 0x59, 0x0b, 0xc2,
	0xb2, 0xb6, 0xae, 0x6d, 0xcc, 0xd6, 0x26, 0xbf, 0x54, 0x27, 0xec, 0xc5, 0xac, 0x7b, 0xe8, 0x9d,
	0xc6, 0x3d, 0xbc, 0x81, 0xe6, 0x95, 0x23, 0xa7, 0x0b, 0x91, 0xa0, 0x2c, 0x2c, 0x4f, 0xf4, 0x1e,
	0x9e, 0x53, 0x9d, 0x27, 0x49, 0xc3, 0xf8, 0x03, 0xad, 0xdd, 0x2a, 0x2e, 0x38, 0x0b, 0x05, 0x18,
	0x87, 0x68, 0xd9, 0x06, 0xce, 0x22, 0x19, 0x3f, 0x71, 0x22, 0x49, 0x24, 0xc7, 0xf5, 0x65, 0x10,
	0x54, 0xba, 0x81, 0x4a, 0x54, 0xf0, 0x1e, 0x9a, 0x8a, 0x57, 0x4b, 0xbd, 0xfe, 0x73, 0xa5, 0x62,
	0xe6, 0xd9, 0x79, 0x33, 0xc6, 0xd9, 0xea, 0x7d, 0xe3, 0xf3, 0x24, 0x5a, 0xe9, 0x69, 0x34, 0x22,
	0xe6, 0x47, 0x20, 0xc4, 0xd8, 0x2b, 0xb9, 0x8a, 0xa6, 0x63, 0xac, 0x43, 0xbd, 0xfe, 0x35, 0x2c,
	0xc6, 0xb5, 0x43, 0x0f, 0x3f, 0x45, 0xb3, 0xaa, 0x2b, 0x2f, 0x39, 0x94, 0x27, 0xd7, 0xb5, 0x8d,
	0x85, 0xca, 0x3f, 0xf9, 0x9d, 0x9f, 0x5e, 0x72, 0x48, 0xb8, 0x33, 0x32, 0xfd, 0x89, 0x5f, 0x6b,
	0xe8, 0x37, 0xc2, 0x79, 0x70, 0xe9, 0x70, 0x22, 0xdd, 0x73, 0x10, 0x8e, 0x12, 0xe2, 0xe9, 0xf7,
	0x94, 0xa7, 0xd4, 0x32, 0xed, 0xe5, 0x13, 0xab, 0xc6, 0xc0, 0x46, 0xc2, 0xeb, 0x5f, 0x9d, 0x83,
	0x82, 0x5d, 0x26, 0xb7, 0xf4, 0xf0, 0x15, 0x2a, 0xc1, 0x05, 0xb8, 0x8e, 0x90, 0xc0, 0x87, 0x4c,
	0xfc, 0xa4, 0x4c, 0xd4, 0xf2, 0x99, 0xa8, 0x5f, 0x80, 0x7b, 0x22, 0x81, 0x0f, 0x19, 0x58, 0x82,
	0x11, 0xf5, 0x1a, 0x42, 0x33, 0x99, 0x9a, 0xf1, 0x52, 0x43, 0xfa, 0xa8, 0x9d, 0x4d, 0x03, 0x74,
	0x86, 0x16, 0x94, 0x3b, 0x8f, 0x46, 0xe0, 0x4a, 0xda, 0x05, 0xb5, 0xaf, 0x0b, 0x95, 0xed, 0xfc,
	0x1b, 0xb2, 0x9b, 0x21, 0xec, 0x79, 0xd9, 0xff, 0xd3, 0xf8, 0x3a, 0x10, 0xae, 0xff, 0x58, 0x9b,
	0x07, 0x20, 0xe1, 0x87, 0x0b, 0xd7, 0x9f, 0x68, 0x1e, 0xa2, 0x88, 0x45, 0x4e, 0x1b, 0x84, 0x20,
	0x3e, 0xa8, 0x34, 0xcd, 0xda, 0x73, 0xaa, 0x78, 0x9c, 0xd4, 0xf0, 0x2b, 0x0d, 0xad, 0x8c, 0x48,
	0x20, 0xeb, 0x48, 0xde, 0x91, 0xe9, 0xd6, 0xef, 0xde, 0x2f, 0x7f, 0x8f, 0x14, 0xeb, 0xa0, 0x60,
	0x2f, 0x93, 0x91, 0x1d, 0xdc, 0x41, 0xbf, 0x0e, 0x65, 0x2f, 0x95, 0x2f, 0x2a, 0xf9, 0x9d, 0xf1,
	0x93, 0x77, 0x2d, 0x8d, 0xe1, 0x46, 0xb5, 0x36, 0x83, 0x8a, 0x89, 0x8e, 0xb1, 0xda, 0x1f, 0xb9,
	0xde, 0x7e, 0x27, 0x91, 0xab, 0xbc, 0x9f, 0x46, 0x4b, 0xd5, 0x58, 0xab, 0x9e, 0x6a, 0x9d, 0x40,
	0xd4, 0xa5, 0x2e, 0xe0, 0x0f, 0x5a, 0x3c, 0xe8, 0x46, 0x8e, 0x55, 0x7c, 0x94, 0xcf, 0xf4, 0xdd,
	0x47, 0x83, 0x7e, 0xfc, 0x40, 0xb4, 0x74, 0xd6, 0x17, 0x36, 0x35, 0xfc, 0x46, 0x43, 0x8b, 0x43,
	0x53, 0x1a, 0xef, 0xe6, 0x95, 0x19, 0x75, 0x5e, 0xe8, 0xf5, 0x7b, 0x52, 0x32, 0x93, 0xf8, 0x9d,
	0x86, 0xf0, 0xcd, 0x51, 0x80, 0xf7, 0xc7, 0xe5, 0x0f, 0x1d, 0x13, 0xfa, 0xc1, 0xfd, 0x41, 0xb7,
	0x78, 0xcd, 0x32, 0x34, 0xbe, 0xd7, 0xa1, 0xa9, 0x33, 0xbe, 0xd7, 0xe1, 0x38, 0x1b, 0x05, 0xfc,
	0x51, 0x43, 0xab, 0x47, 0x8c, 0xb5, 0x3a, 0xbc, 0xde, 0x6c, 0x26, 0x33, 0x6f, 0x3f, 0x16, 0x69,
	0xa4, 0x97, 0x20, 0xfc, 0x38, 0x9f, 0xd8, 0x5d, 0xac, 0xcc, 0xbf, 0xfd, 0x90, 0xc8, 0xec, 0x4b,
	0x74, 0xfd, 0x53, 0xb5, 0x74, 0x8d, 0x4a, 0x04, 0x08, 0xa7, 0xc2, 0x74, 0x59, 0xbb, 0xf6, 0x56,
	0x43, 0x9b, 0x2e, 0x6b, 0xe7, 0xd2, 0xad, 0xe1, 0x81, 0x3f, 0x7a, 0x23, 0xbe, 0xb9, 0x35, 0xb4,
	0xe7, 0xcf, 0x52, 0x86, 0xcf, 0x02, 0x12, 0xfa, 0x26, 0x8b, 0x7c, 0xcb, 0x87, 0x50, 0xdd, 0xeb,
	0xac, 0x9e, 0xea, 0xf7, 0xdd, 0x16, 0xb7, 0x07, 0xca, 0x67, 0x45, 0x45, 0xf9, 0xfb, 0x5b, 0x00,
	0x00, 0x00, 0xff, 0xff, 0xfe, 0x0b, 0x43, 0x7b, 0x46, 0x0b, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// AgentEndpointServiceClient is the client API for AgentEndpointService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type AgentEndpointServiceClient interface {
	// Stream established by client to receive Task notifications.
	// This method is called by an agent and not an active developer method.
	ReceiveTaskNotification(ctx context.Context, in *ReceiveTaskNotificationRequest, opts ...grpc.CallOption) (AgentEndpointService_ReceiveTaskNotificationClient, error)
	// Signals the start of a task execution and returns the task info.
	// This method is called by an agent and not an active developer method.
	ReportTaskStart(ctx context.Context, in *ReportTaskStartRequest, opts ...grpc.CallOption) (*ReportTaskStartResponse, error)
	// Signals an intermediary progress checkpoint in task execution.
	// This method is called by an agent and not an active developer method.
	ReportTaskProgress(ctx context.Context, in *ReportTaskProgressRequest, opts ...grpc.CallOption) (*ReportTaskProgressResponse, error)
	// Signals that the task execution is complete and optionally returns the next
	// task.
	// This method is called by an agent and not an active developer method.
	ReportTaskComplete(ctx context.Context, in *ReportTaskCompleteRequest, opts ...grpc.CallOption) (*ReportTaskCompleteResponse, error)
	// Looks up the effective guest policies for an instance.
	LookupEffectiveGuestPolicies(ctx context.Context, in *LookupEffectiveGuestPoliciesRequest, opts ...grpc.CallOption) (*LookupEffectiveGuestPoliciesResponse, error)
}

type agentEndpointServiceClient struct {
	cc *grpc.ClientConn
}

func NewAgentEndpointServiceClient(cc *grpc.ClientConn) AgentEndpointServiceClient {
	return &agentEndpointServiceClient{cc}
}

func (c *agentEndpointServiceClient) ReceiveTaskNotification(ctx context.Context, in *ReceiveTaskNotificationRequest, opts ...grpc.CallOption) (AgentEndpointService_ReceiveTaskNotificationClient, error) {
	stream, err := c.cc.NewStream(ctx, &_AgentEndpointService_serviceDesc.Streams[0], "/google.cloud.osconfig.agentendpoint.v1alpha1.AgentEndpointService/ReceiveTaskNotification", opts...)
	if err != nil {
		return nil, err
	}
	x := &agentEndpointServiceReceiveTaskNotificationClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type AgentEndpointService_ReceiveTaskNotificationClient interface {
	Recv() (*ReceiveTaskNotificationResponse, error)
	grpc.ClientStream
}

type agentEndpointServiceReceiveTaskNotificationClient struct {
	grpc.ClientStream
}

func (x *agentEndpointServiceReceiveTaskNotificationClient) Recv() (*ReceiveTaskNotificationResponse, error) {
	m := new(ReceiveTaskNotificationResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *agentEndpointServiceClient) ReportTaskStart(ctx context.Context, in *ReportTaskStartRequest, opts ...grpc.CallOption) (*ReportTaskStartResponse, error) {
	out := new(ReportTaskStartResponse)
	err := c.cc.Invoke(ctx, "/google.cloud.osconfig.agentendpoint.v1alpha1.AgentEndpointService/ReportTaskStart", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *agentEndpointServiceClient) ReportTaskProgress(ctx context.Context, in *ReportTaskProgressRequest, opts ...grpc.CallOption) (*ReportTaskProgressResponse, error) {
	out := new(ReportTaskProgressResponse)
	err := c.cc.Invoke(ctx, "/google.cloud.osconfig.agentendpoint.v1alpha1.AgentEndpointService/ReportTaskProgress", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *agentEndpointServiceClient) ReportTaskComplete(ctx context.Context, in *ReportTaskCompleteRequest, opts ...grpc.CallOption) (*ReportTaskCompleteResponse, error) {
	out := new(ReportTaskCompleteResponse)
	err := c.cc.Invoke(ctx, "/google.cloud.osconfig.agentendpoint.v1alpha1.AgentEndpointService/ReportTaskComplete", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *agentEndpointServiceClient) LookupEffectiveGuestPolicies(ctx context.Context, in *LookupEffectiveGuestPoliciesRequest, opts ...grpc.CallOption) (*LookupEffectiveGuestPoliciesResponse, error) {
	out := new(LookupEffectiveGuestPoliciesResponse)
	err := c.cc.Invoke(ctx, "/google.cloud.osconfig.agentendpoint.v1alpha1.AgentEndpointService/LookupEffectiveGuestPolicies", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// AgentEndpointServiceServer is the server API for AgentEndpointService service.
type AgentEndpointServiceServer interface {
	// Stream established by client to receive Task notifications.
	// This method is called by an agent and not an active developer method.
	ReceiveTaskNotification(*ReceiveTaskNotificationRequest, AgentEndpointService_ReceiveTaskNotificationServer) error
	// Signals the start of a task execution and returns the task info.
	// This method is called by an agent and not an active developer method.
	ReportTaskStart(context.Context, *ReportTaskStartRequest) (*ReportTaskStartResponse, error)
	// Signals an intermediary progress checkpoint in task execution.
	// This method is called by an agent and not an active developer method.
	ReportTaskProgress(context.Context, *ReportTaskProgressRequest) (*ReportTaskProgressResponse, error)
	// Signals that the task execution is complete and optionally returns the next
	// task.
	// This method is called by an agent and not an active developer method.
	ReportTaskComplete(context.Context, *ReportTaskCompleteRequest) (*ReportTaskCompleteResponse, error)
	// Looks up the effective guest policies for an instance.
	LookupEffectiveGuestPolicies(context.Context, *LookupEffectiveGuestPoliciesRequest) (*LookupEffectiveGuestPoliciesResponse, error)
}

// UnimplementedAgentEndpointServiceServer can be embedded to have forward compatible implementations.
type UnimplementedAgentEndpointServiceServer struct {
}

func (*UnimplementedAgentEndpointServiceServer) ReceiveTaskNotification(req *ReceiveTaskNotificationRequest, srv AgentEndpointService_ReceiveTaskNotificationServer) error {
	return status.Errorf(codes.Unimplemented, "method ReceiveTaskNotification not implemented")
}
func (*UnimplementedAgentEndpointServiceServer) ReportTaskStart(ctx context.Context, req *ReportTaskStartRequest) (*ReportTaskStartResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReportTaskStart not implemented")
}
func (*UnimplementedAgentEndpointServiceServer) ReportTaskProgress(ctx context.Context, req *ReportTaskProgressRequest) (*ReportTaskProgressResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReportTaskProgress not implemented")
}
func (*UnimplementedAgentEndpointServiceServer) ReportTaskComplete(ctx context.Context, req *ReportTaskCompleteRequest) (*ReportTaskCompleteResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReportTaskComplete not implemented")
}
func (*UnimplementedAgentEndpointServiceServer) LookupEffectiveGuestPolicies(ctx context.Context, req *LookupEffectiveGuestPoliciesRequest) (*LookupEffectiveGuestPoliciesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method LookupEffectiveGuestPolicies not implemented")
}

func RegisterAgentEndpointServiceServer(s *grpc.Server, srv AgentEndpointServiceServer) {
	s.RegisterService(&_AgentEndpointService_serviceDesc, srv)
}

func _AgentEndpointService_ReceiveTaskNotification_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(ReceiveTaskNotificationRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(AgentEndpointServiceServer).ReceiveTaskNotification(m, &agentEndpointServiceReceiveTaskNotificationServer{stream})
}

type AgentEndpointService_ReceiveTaskNotificationServer interface {
	Send(*ReceiveTaskNotificationResponse) error
	grpc.ServerStream
}

type agentEndpointServiceReceiveTaskNotificationServer struct {
	grpc.ServerStream
}

func (x *agentEndpointServiceReceiveTaskNotificationServer) Send(m *ReceiveTaskNotificationResponse) error {
	return x.ServerStream.SendMsg(m)
}

func _AgentEndpointService_ReportTaskStart_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReportTaskStartRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AgentEndpointServiceServer).ReportTaskStart(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/google.cloud.osconfig.agentendpoint.v1alpha1.AgentEndpointService/ReportTaskStart",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AgentEndpointServiceServer).ReportTaskStart(ctx, req.(*ReportTaskStartRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AgentEndpointService_ReportTaskProgress_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReportTaskProgressRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AgentEndpointServiceServer).ReportTaskProgress(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/google.cloud.osconfig.agentendpoint.v1alpha1.AgentEndpointService/ReportTaskProgress",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AgentEndpointServiceServer).ReportTaskProgress(ctx, req.(*ReportTaskProgressRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AgentEndpointService_ReportTaskComplete_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReportTaskCompleteRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AgentEndpointServiceServer).ReportTaskComplete(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/google.cloud.osconfig.agentendpoint.v1alpha1.AgentEndpointService/ReportTaskComplete",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AgentEndpointServiceServer).ReportTaskComplete(ctx, req.(*ReportTaskCompleteRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AgentEndpointService_LookupEffectiveGuestPolicies_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LookupEffectiveGuestPoliciesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AgentEndpointServiceServer).LookupEffectiveGuestPolicies(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/google.cloud.osconfig.agentendpoint.v1alpha1.AgentEndpointService/LookupEffectiveGuestPolicies",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AgentEndpointServiceServer).LookupEffectiveGuestPolicies(ctx, req.(*LookupEffectiveGuestPoliciesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _AgentEndpointService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "google.cloud.osconfig.agentendpoint.v1alpha1.AgentEndpointService",
	HandlerType: (*AgentEndpointServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ReportTaskStart",
			Handler:    _AgentEndpointService_ReportTaskStart_Handler,
		},
		{
			MethodName: "ReportTaskProgress",
			Handler:    _AgentEndpointService_ReportTaskProgress_Handler,
		},
		{
			MethodName: "ReportTaskComplete",
			Handler:    _AgentEndpointService_ReportTaskComplete_Handler,
		},
		{
			MethodName: "LookupEffectiveGuestPolicies",
			Handler:    _AgentEndpointService_LookupEffectiveGuestPolicies_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "ReceiveTaskNotification",
			Handler:       _AgentEndpointService_ReceiveTaskNotification_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "google/cloud/osconfig/agentendpoint/v1alpha1/agentendpoint.proto",
}
